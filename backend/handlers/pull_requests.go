package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/brian/config-generation/backend/models"
	"github.com/go-chi/chi/v5"
)

type PullRequestHandler struct {
	DB *sql.DB
}

func (h *PullRequestHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	var req models.CreatePullRequestRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required", "validation")
		return
	}
	if req.ObjectType != "global_values" {
		writeError(w, http.StatusBadRequest, "unsupported object_type; only global_values is supported", "validation")
		return
	}
	if req.GlobalValuesName == nil || *req.GlobalValuesName == "" {
		writeError(w, http.StatusBadRequest, "global_values_name is required for global_values changes", "validation")
		return
	}
	if req.ProposedPayload == "" {
		writeError(w, http.StatusBadRequest, "proposed_payload is required", "validation")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tx.Rollback()

	// Look up the latest group version for the named group. The PR pins this
	// version as its base for conflict detection at merge.
	var baseGroupVersionID int64
	err = tx.QueryRowContext(r.Context(), `
		SELECT ggv.id
		FROM global_values_groups g
		JOIN global_values_group_versions ggv ON ggv.group_id = g.id
		WHERE g.name = $1
		ORDER BY ggv.version_id DESC LIMIT 1
	`, *req.GlobalValuesName).Scan(&baseGroupVersionID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "global values group not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	// Create the pull request row.
	var pr models.PullRequest
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO pull_requests (project_id, global_values_name, base_group_version_id, author_id, title, description, status)
		VALUES (NULL, $1, $2, $3, $4, $5, 'open')
		RETURNING id, project_id, global_values_name, base_project_version_id, base_group_version_id,
		          author_id, title, description, status, is_conflicted,
		          created_at, updated_at, merged_at, closed_at
	`, *req.GlobalValuesName, baseGroupVersionID, user.UserID, req.Title, req.Description).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.BaseProjectVersionID, &pr.BaseGroupVersionID,
		&pr.AuthorID, &pr.Title, &pr.Description, &pr.Status, &pr.IsConflicted,
		&pr.CreatedAt, &pr.UpdatedAt, &pr.MergedAt, &pr.ClosedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create pull request", "internal")
		return
	}

	// Create the pr_changes row.
	var change models.PRChange
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO pr_changes (pr_id, object_type, operation, global_values_name, proposed_payload)
		VALUES ($1, 'global_values', 'update', $2, $3)
		RETURNING id, pr_id, object_type, operation, project_id, template_name,
		          environment_name, global_values_name, proposed_payload, created_at
	`, pr.ID, *req.GlobalValuesName, req.ProposedPayload).Scan(
		&change.ID, &change.PRID, &change.ObjectType, &change.Operation, &change.ProjectID,
		&change.TemplateName, &change.EnvironmentName, &change.GlobalValuesName,
		&change.ProposedPayload, &change.CreatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create pr change", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, msgFailedToCommit, "internal")
		return
	}

	pr.Changes = []models.PRChange{change}
	writeJSON(w, http.StatusCreated, pr)
}

// loadApprovalCondition returns the approval condition string for a PR.
// For global values PRs, it comes from the global_values entry's v1 row.
// For project PRs, it comes from the project.
func (h *PullRequestHandler) loadApprovalCondition(ctx context.Context, pr *models.PullRequest) (string, error) {
	if pr.GlobalValuesName != nil {
		var cond string
		err := h.DB.QueryRowContext(ctx, `
			SELECT approval_condition FROM global_values_groups WHERE name = $1
		`, *pr.GlobalValuesName).Scan(&cond)
		if err != nil {
			return "", err
		}
		return cond, nil
	}
	if pr.ProjectID != nil {
		var cond string
		err := h.DB.QueryRowContext(ctx, `
			SELECT approval_condition FROM projects WHERE id = $1
		`, *pr.ProjectID).Scan(&cond)
		if err != nil {
			return "", err
		}
		return cond, nil
	}
	return "1 x gv_group_admin", nil
}

// loadApprovals fetches active (non-withdrawn) approvals for a PR.
func (h *PullRequestHandler) loadApprovals(ctx context.Context, prID int64) ([]models.PRApproval, error) {
	rows, err := h.DB.QueryContext(ctx, `
		SELECT id, pr_id, user_id, approved_at, withdrawn_at
		FROM pr_approvals
		WHERE pr_id = $1 AND withdrawn_at IS NULL
		ORDER BY approved_at
	`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var approvals []models.PRApproval
	for rows.Next() {
		var a models.PRApproval
		if err := rows.Scan(&a.ID, &a.PRID, &a.UserID, &a.ApprovedAt, &a.WithdrawnAt); err != nil {
			return nil, err
		}
		approvals = append(approvals, a)
	}
	if approvals == nil {
		approvals = []models.PRApproval{}
	}
	return approvals, rows.Err()
}

// roleRequirement represents a parsed "N x role_name" requirement.
type roleRequirement struct {
	Count    int
	RoleName string
}

// parseApprovalCondition parses a simple approval condition like
// "1 x gv_group_admin" or "1 x admin AND 1 x reviewer".
// Only supports AND-joined requirements (no OR or parentheses yet).
func parseApprovalCondition(condition string) []roleRequirement {
	var reqs []roleRequirement
	re := regexp.MustCompile(`(\d+)\s*x\s*(\S+)`)
	matches := re.FindAllStringSubmatch(condition, -1)
	for _, m := range matches {
		count, _ := strconv.Atoi(m[1])
		reqs = append(reqs, roleRequirement{Count: count, RoleName: m[2]})
	}
	return reqs
}

// loadApproverRoles returns each approver's set of global role names.
func (h *PullRequestHandler) loadApproverRoles(ctx context.Context, approvals []models.PRApproval) (map[int64][]string, error) {
	approverRoles := make(map[int64][]string, len(approvals))
	for _, a := range approvals {
		roles, err := h.userRoleNames(ctx, a.UserID)
		if err != nil {
			return nil, err
		}
		approverRoles[a.UserID] = roles
	}
	return approverRoles, nil
}

// userRoleNames fetches the global role names held by a user.
func (h *PullRequestHandler) userRoleNames(ctx context.Context, userID int64) ([]string, error) {
	rows, err := h.DB.QueryContext(ctx,
		`SELECT r.name FROM roles r
		 JOIN user_roles ur ON ur.role_id = r.id
		 WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		roles = append(roles, name)
	}
	return roles, rows.Err()
}

// countApproversWithRole returns how many approvers hold the named role.
func countApproversWithRole(approverRoles map[int64][]string, reqName string) int {
	n := 0
	for _, roles := range approverRoles {
		for _, r := range roles {
			if r == reqName {
				n++
				break
			}
		}
	}
	return n
}

// requirementsMet reports whether the set of approverRoles satisfies all
// requirements (AND) or any one of them (OR).
func requirementsMet(reqs []roleRequirement, approverRoles map[int64][]string, isAnd bool) bool {
	if isAnd {
		for _, req := range reqs {
			if countApproversWithRole(approverRoles, req.RoleName) < req.Count {
				return false
			}
		}
		return true
	}
	for _, req := range reqs {
		if countApproversWithRole(approverRoles, req.RoleName) >= req.Count {
			return true
		}
	}
	return false
}

// checkApprovalConditionMet checks if the approval condition is satisfied
// by the current approvals. It queries role membership for each approver.
func (h *PullRequestHandler) checkApprovalConditionMet(ctx context.Context, pr *models.PullRequest, condition string, approvals []models.PRApproval) (bool, error) {
	reqs := parseApprovalCondition(condition)
	if len(reqs) == 0 {
		return true, nil
	}

	approverRoles, err := h.loadApproverRoles(ctx, approvals)
	if err != nil {
		return false, err
	}

	// AND semantics: all requirements must be met (the default and for a single
	// requirement). OR: at least one.
	isAnd := strings.Contains(strings.ToUpper(condition), "AND") || len(reqs) == 1
	return requirementsMet(reqs, approverRoles, isAnd), nil
}

func (h *PullRequestHandler) Approve(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	prID, err := urlParamInt64(r, "prID")
	if err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidPRID, "bad_request")
		return
	}

	// Load the PR.
	var pr models.PullRequest
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, project_id, global_values_name, author_id, title, description, status,
		       is_conflicted, created_at, updated_at, merged_at, closed_at
		FROM pull_requests WHERE id = $1
	`, prID).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgPullRequestNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	// PR must be open or approved.
	if pr.Status != "open" && pr.Status != "approved" {
		writeError(w, http.StatusConflict, "pull request is not open for approval", "conflict")
		return
	}

	// Insert or update the approval (upsert: clear withdrawn_at if re-approving).
	_, err = h.DB.ExecContext(r.Context(), `
		INSERT INTO pr_approvals (pr_id, user_id) VALUES ($1, $2)
		ON CONFLICT (pr_id, user_id) DO UPDATE SET approved_at = NOW(), withdrawn_at = NULL
	`, prID, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record approval", "internal")
		return
	}

	// Check if approval condition is now met.
	condition, err := h.loadApprovalCondition(r.Context(), &pr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load approval condition", "internal")
		return
	}

	approvals, err := h.loadApprovals(r.Context(), prID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load approvals", "internal")
		return
	}

	met, err := h.checkApprovalConditionMet(r.Context(), &pr, condition, approvals)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to evaluate approval condition", "internal")
		return
	}

	// Auto-transition to approved if condition is met.
	if met && pr.Status == "open" {
		_, err = h.DB.ExecContext(r.Context(), `
			UPDATE pull_requests SET status = 'approved', updated_at = NOW() WHERE id = $1
		`, prID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update status", "internal")
			return
		}
		pr.Status = "approved"
	}

	pr.ApprovalCondition = condition
	pr.Approvals = approvals
	writeJSON(w, http.StatusOK, pr)
}

// reevaluateApprovedAfterWithdraw recomputes whether an approved PR still
// satisfies its approval condition after one approver withdraws; if not, it
// transitions the PR back to "open" and refreshes the approval-condition and
// approvals fields on pr. A no-op when pr.Status != "approved". Extracted from
// WithdrawApproval to keep its cognitive complexity within limit.
func (h *PullRequestHandler) reevaluateApprovedAfterWithdraw(ctx context.Context, pr *models.PullRequest) error {
	if pr.Status != "approved" {
		return nil
	}
	condition, err := h.loadApprovalCondition(ctx, pr)
	if err != nil {
		return err
	}
	approvals, err := h.loadApprovals(ctx, pr.ID)
	if err != nil {
		return err
	}
	met, err := h.checkApprovalConditionMet(ctx, pr, condition, approvals)
	if err != nil {
		return err
	}
	if !met {
		if _, err := h.DB.ExecContext(ctx, `
			UPDATE pull_requests SET status = 'open', updated_at = NOW() WHERE id = $1
		`, pr.ID); err != nil {
			return err
		}
		pr.Status = "open"
	}
	pr.ApprovalCondition = condition
	pr.Approvals = approvals
	return nil
}

func (h *PullRequestHandler) WithdrawApproval(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	prID, err := urlParamInt64(r, "prID")
	if err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidPRID, "bad_request")
		return
	}

	result, err := h.DB.ExecContext(r.Context(), `
		UPDATE pr_approvals SET withdrawn_at = NOW()
		WHERE pr_id = $1 AND user_id = $2 AND withdrawn_at IS NULL
	`, prID, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeError(w, http.StatusNotFound, "no active approval found", "not_found")
		return
	}

	// If the PR was approved, re-evaluate: it may need to go back to open.
	var pr models.PullRequest
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, project_id, global_values_name, author_id, title, description, status,
		       is_conflicted, created_at, updated_at, merged_at, closed_at
		FROM pull_requests WHERE id = $1
	`, prID).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	if err := h.reevaluateApprovedAfterWithdraw(r.Context(), &pr); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

func (h *PullRequestHandler) Get(w http.ResponseWriter, r *http.Request) {
	prID, err := urlParamInt64(r, "prID")
	if err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidPRID, "bad_request")
		return
	}

	var pr models.PullRequest
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, project_id, global_values_name, author_id, title, description, status,
		       is_conflicted, created_at, updated_at, merged_at, closed_at
		FROM pull_requests WHERE id = $1
	`, prID).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgPullRequestNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	pr.Changes, err = h.loadChanges(r.Context(), pr.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	// Load approval condition.
	condition, err := h.loadApprovalCondition(r.Context(), &pr)
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "failed to load approval condition", "internal")
		return
	}
	pr.ApprovalCondition = condition

	// Load approvals.
	approvals, err := h.loadApprovals(r.Context(), pr.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load approvals", "internal")
		return
	}
	pr.Approvals = approvals

	writeJSON(w, http.StatusOK, pr)
}

func (h *PullRequestHandler) Close(w http.ResponseWriter, r *http.Request) {
	prID, err := urlParamInt64(r, "prID")
	if err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidPRID, "bad_request")
		return
	}

	var currentStatus string
	err = h.DB.QueryRowContext(r.Context(),
		`SELECT status FROM pull_requests WHERE id = $1`, prID,
	).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgPullRequestNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	if currentStatus != "draft" && currentStatus != "open" && currentStatus != "approved" {
		writeError(w, http.StatusConflict, "pull request cannot be closed in its current state", "conflict")
		return
	}

	var pr models.PullRequest
	err = h.DB.QueryRowContext(r.Context(), `
		UPDATE pull_requests
		SET status = 'closed', closed_at = NOW(), updated_at = NOW()
		WHERE id = $1
		RETURNING id, project_id, global_values_name, author_id, title, description, status,
		          is_conflicted, created_at, updated_at, merged_at, closed_at
	`, prID).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to close pull request", "internal")
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

// loadAndLockPRForMerge fetches the PR with a row lock and validates that it
// is eligible to be merged by the given user.
func loadAndLockPRForMerge(ctx context.Context, tx *sql.Tx, prID, userID int64) (models.PullRequest, int, string, string) {
	var pr models.PullRequest
	err := tx.QueryRowContext(ctx, `
		SELECT id, project_id, global_values_name, base_project_version_id, base_group_version_id,
		       author_id, title, description, status, is_conflicted,
		       created_at, updated_at, merged_at, closed_at
		FROM pull_requests WHERE id = $1 FOR UPDATE
	`, prID).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.BaseProjectVersionID, &pr.BaseGroupVersionID,
		&pr.AuthorID, &pr.Title, &pr.Description, &pr.Status, &pr.IsConflicted,
		&pr.CreatedAt, &pr.UpdatedAt, &pr.MergedAt, &pr.ClosedAt,
	)
	if err == sql.ErrNoRows {
		return pr, http.StatusNotFound, msgPullRequestNotFound, "not_found"
	}
	if err != nil {
		return pr, http.StatusInternalServerError, msgDatabaseError, "internal"
	}
	if pr.AuthorID != userID {
		return pr, http.StatusForbidden, "only the PR author can merge", "forbidden"
	}
	if pr.Status != "approved" {
		return pr, http.StatusConflict, "pull request must be approved before merging", "conflict"
	}
	if pr.IsConflicted {
		return pr, http.StatusConflict, "pull request has conflicts and cannot be merged", "conflict"
	}
	return pr, 0, "", ""
}

// loadPRChangesTx reads all pr_changes rows for the PR inside a transaction.
func loadPRChangesTx(ctx context.Context, tx *sql.Tx, prID int64) ([]models.PRChange, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, pr_id, object_type, operation, project_id, template_name,
		       environment_name, global_values_name, proposed_payload, created_at
		FROM pr_changes WHERE pr_id = $1
	`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var changes []models.PRChange
	for rows.Next() {
		var c models.PRChange
		if err := rows.Scan(
			&c.ID, &c.PRID, &c.ObjectType, &c.Operation, &c.ProjectID, &c.TemplateName,
			&c.EnvironmentName, &c.GlobalValuesName, &c.ProposedPayload, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		changes = append(changes, c)
	}
	return changes, rows.Err()
}

// applyPRChanges produces a single new snapshot (project_version for project
// PRs, group_version for GV PRs) bundling every staged change. Environment
// creates land in the environments table first so values changes can resolve
// the env name to an id while building the new snapshot.
func applyPRChanges(ctx context.Context, tx *sql.Tx, pr *models.PullRequest, changes []models.PRChange, authorID int64, commitMsg string) (int, string, string) {
	// Environment creates first (needed so values rows can reference the env_id).
	for _, c := range changes {
		if c.ObjectType == "environment" && c.Operation != "delete" {
			if err := applyEnvironmentCreate(ctx, tx, c, authorID); err != nil {
				return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
			}
		}
	}

	if pr.ProjectID != nil {
		if status, msg, code := applyProjectSnapshot(ctx, tx, pr, changes, authorID, commitMsg); status != 0 {
			return status, msg, code
		}
	} else if pr.GlobalValuesName != nil {
		if status, msg, code := applyGroupSnapshot(ctx, tx, pr, changes, authorID, commitMsg); status != 0 {
			return status, msg, code
		}
	}

	// Environment deletes last (after the new snapshot leaves the env unlinked).
	for _, c := range changes {
		if c.ObjectType == "environment" && c.Operation == "delete" {
			if err := applyEnvironmentDelete(ctx, tx, c); err != nil {
				return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
			}
		}
	}
	return 0, "", ""
}

func applyEnvironmentCreate(ctx context.Context, tx *sql.Tx, c models.PRChange, authorID int64) error {
	if c.ProjectID == nil || c.EnvironmentName == nil {
		return nil
	}
	var envReq models.CreateEnvironmentRequest
	if json.Unmarshal([]byte(c.ProposedPayload), &envReq) != nil {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO environments (project_id, name, description, created_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (project_id, name) DO NOTHING
	`, *c.ProjectID, envReq.Name, envReq.Description, authorID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO env_admins (environment_id, user_id, granted_by)
		SELECT id, $3, $3 FROM environments WHERE project_id = $1 AND name = $2
		ON CONFLICT (environment_id, user_id) DO NOTHING
	`, *c.ProjectID, envReq.Name, authorID)
	return err
}

// applyEnvironmentDelete removes an env and all its historical content rows.
// project_version_values link rows referencing those content rows are deleted
// too, so older project_versions stop resolving values for the dropped env.
// This is destructive of history for the env; that matches the prior model.
func applyEnvironmentDelete(ctx context.Context, tx *sql.Tx, c models.PRChange) error {
	if c.ProjectID == nil || c.EnvironmentName == nil {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM project_version_values
		WHERE values_row_id IN (
			SELECT v.id FROM project_config_values v
			JOIN environments e ON e.id = v.environment_id
			WHERE v.project_id = $1 AND e.name = $2
		)
	`, *c.ProjectID, *c.EnvironmentName); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM project_config_values
		WHERE project_id = $1
		  AND environment_id = (SELECT id FROM environments WHERE project_id = $1 AND name = $2)
	`, *c.ProjectID, *c.EnvironmentName); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM env_admins
		WHERE environment_id = (SELECT id FROM environments WHERE project_id = $1 AND name = $2)
	`, *c.ProjectID, *c.EnvironmentName); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		DELETE FROM environments WHERE project_id = $1 AND name = $2
	`, *c.ProjectID, *c.EnvironmentName)
	return err
}

// applyProjectSnapshot creates one new project_versions row, copies forward
// every (template_name → row_id) and (environment_id → row_id) link from the
// PR's base, then overlays each PR change (insert new content row + upsert /
// delete the link). Conflict check: if the project's latest version no longer
// matches the PR's base, bail with 409.
func applyProjectSnapshot(ctx context.Context, tx *sql.Tx, pr *models.PullRequest, changes []models.PRChange, authorID int64, commitMsg string) (int, string, string) {
	projectID := *pr.ProjectID

	// Lock the project's version sequence.
	var latestPVID *int64
	var latestOrd *int
	if err := tx.QueryRowContext(ctx, `
		SELECT id, version_id FROM project_versions
		WHERE project_id = $1
		ORDER BY version_id DESC LIMIT 1
		FOR UPDATE
	`, projectID).Scan(&latestPVID, &latestOrd); err != nil && err != sql.ErrNoRows {
		return http.StatusInternalServerError, msgDatabaseError, "internal"
	}

	// Conflict: base mismatched the current head.
	if pr.BaseProjectVersionID != nil && latestPVID != nil && *latestPVID != *pr.BaseProjectVersionID {
		_, _ = tx.ExecContext(ctx, `UPDATE pull_requests SET is_conflicted = true WHERE id = $1`, pr.ID)
		return http.StatusConflict, "pull request base is stale; rebase against the latest project version", "conflict"
	}

	nextOrd := 1
	if latestOrd != nil {
		nextOrd = *latestOrd + 1
	}

	var newPVID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO project_versions (project_id, version_id, parent_version_id, commit_message, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, projectID, nextOrd, latestPVID, commitMsg, authorID).Scan(&newPVID); err != nil {
		return http.StatusInternalServerError, "failed to create project version", "internal"
	}

	// Copy forward existing links from the base version (no-op if no base).
	if latestPVID != nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO project_version_templates (project_version_id, template_name, template_row_id)
			SELECT $1, template_name, template_row_id
			FROM project_version_templates WHERE project_version_id = $2
		`, newPVID, *latestPVID); err != nil {
			return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO project_version_values (project_version_id, environment_id, values_row_id)
			SELECT $1, environment_id, values_row_id
			FROM project_version_values WHERE project_version_id = $2
		`, newPVID, *latestPVID); err != nil {
			return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
		}
	}

	for _, c := range changes {
		switch c.ObjectType {
		case "template":
			if status, msg, code := applyTemplateChangeToSnapshot(ctx, tx, c, newPVID, authorID, commitMsg); status != 0 {
				return status, msg, code
			}
		case "values":
			if status, msg, code := applyValuesChangeToSnapshot(ctx, tx, c, newPVID, authorID, commitMsg); status != 0 {
				return status, msg, code
			}
		}
	}
	return 0, "", ""
}

func applyTemplateChangeToSnapshot(ctx context.Context, tx *sql.Tx, c models.PRChange, newPVID int64, authorID int64, commitMsg string) (int, string, string) {
	if c.ProjectID == nil || c.TemplateName == nil {
		return 0, "", ""
	}
	if c.Operation == "delete" {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM project_version_templates
			WHERE project_version_id = $1 AND template_name = $2
		`, newPVID, *c.TemplateName); err != nil {
			return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
		}
		return 0, "", ""
	}
	// New content row with a monotonic per-template version_id for traceability.
	var nextContentVer int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_id), 0) + 1 FROM project_config_templates
		WHERE project_id = $1 AND template_name = $2
	`, *c.ProjectID, *c.TemplateName).Scan(&nextContentVer); err != nil {
		return http.StatusInternalServerError, msgDatabaseError, "internal"
	}
	var rowID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO project_config_templates (project_id, template_name, version_id, body, commit_message, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, *c.ProjectID, *c.TemplateName, nextContentVer, c.ProposedPayload, commitMsg, authorID).Scan(&rowID); err != nil {
		return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_version_templates (project_version_id, template_name, template_row_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_version_id, template_name) DO UPDATE SET template_row_id = EXCLUDED.template_row_id
	`, newPVID, *c.TemplateName, rowID); err != nil {
		return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
	}
	return 0, "", ""
}

func applyValuesChangeToSnapshot(ctx context.Context, tx *sql.Tx, c models.PRChange, newPVID int64, authorID int64, commitMsg string) (int, string, string) {
	if c.ProjectID == nil || c.EnvironmentName == nil {
		return 0, "", ""
	}
	var envID int64
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM environments WHERE project_id = $1 AND name = $2
	`, *c.ProjectID, *c.EnvironmentName).Scan(&envID)
	if err == sql.ErrNoRows {
		if c.Operation == "delete" {
			return 0, "", ""
		}
		return http.StatusInternalServerError, "environment not found during merge", "internal"
	}
	if err != nil {
		return http.StatusInternalServerError, msgDatabaseError, "internal"
	}
	if c.Operation == "delete" {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM project_version_values
			WHERE project_version_id = $1 AND environment_id = $2
		`, newPVID, envID); err != nil {
			return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
		}
		return 0, "", ""
	}
	var nextContentVer int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_id), 0) + 1 FROM project_config_values
		WHERE project_id = $1 AND environment_id = $2
	`, *c.ProjectID, envID).Scan(&nextContentVer); err != nil {
		return http.StatusInternalServerError, msgDatabaseError, "internal"
	}
	var rowID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO project_config_values (project_id, environment_id, version_id, payload, commit_message, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, *c.ProjectID, envID, nextContentVer, c.ProposedPayload, commitMsg, authorID).Scan(&rowID); err != nil {
		return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_version_values (project_version_id, environment_id, values_row_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_version_id, environment_id) DO UPDATE SET values_row_id = EXCLUDED.values_row_id
	`, newPVID, envID, rowID); err != nil {
		return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
	}
	return 0, "", ""
}

// applyGroupSnapshot creates one new global_values_group_versions row, copies
// forward every entry from the PR's base group version, then writes new
// content rows + upserts entries for each global_values change in the PR.
func applyGroupSnapshot(ctx context.Context, tx *sql.Tx, pr *models.PullRequest, changes []models.PRChange, authorID int64, commitMsg string) (int, string, string) {
	groupName := *pr.GlobalValuesName

	var groupID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT id FROM global_values_groups WHERE name = $1 FOR UPDATE
	`, groupName).Scan(&groupID); err != nil {
		return http.StatusInternalServerError, msgDatabaseError, "internal"
	}

	var latestGGVID *int64
	var latestOrd *int
	if err := tx.QueryRowContext(ctx, `
		SELECT id, version_id FROM global_values_group_versions
		WHERE group_id = $1
		ORDER BY version_id DESC LIMIT 1
	`, groupID).Scan(&latestGGVID, &latestOrd); err != nil && err != sql.ErrNoRows {
		return http.StatusInternalServerError, msgDatabaseError, "internal"
	}

	if pr.BaseGroupVersionID != nil && latestGGVID != nil && *latestGGVID != *pr.BaseGroupVersionID {
		_, _ = tx.ExecContext(ctx, `UPDATE pull_requests SET is_conflicted = true WHERE id = $1`, pr.ID)
		return http.StatusConflict, "pull request base is stale; rebase against the latest group version", "conflict"
	}

	nextOrd := 1
	if latestOrd != nil {
		nextOrd = *latestOrd + 1
	}

	var newGGVID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO global_values_group_versions (group_id, version_id, parent_version_id, commit_message, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, groupID, nextOrd, latestGGVID, commitMsg, authorID).Scan(&newGGVID); err != nil {
		return http.StatusInternalServerError, "failed to create group version", "internal"
	}

	if latestGGVID != nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO global_values_group_version_entries (group_version_id, gv_name, gv_row_id)
			SELECT $1, gv_name, gv_row_id
			FROM global_values_group_version_entries WHERE group_version_id = $2
		`, newGGVID, *latestGGVID); err != nil {
			return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
		}
	}

	for _, c := range changes {
		if c.ObjectType != "global_values" || c.GlobalValuesName == nil {
			continue
		}
		// Treat the proposed payload as the *entire* new payload for the named
		// value in this group. (PRs in the current shape carry a single change
		// per named value.)
		var rowID int64
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO global_values (group_id, name, payload, commit_message, created_by)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, groupID, *c.GlobalValuesName, c.ProposedPayload, commitMsg, authorID).Scan(&rowID); err != nil {
			return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO global_values_group_version_entries (group_version_id, gv_name, gv_row_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (group_version_id, gv_name) DO UPDATE SET gv_row_id = EXCLUDED.gv_row_id
		`, newGGVID, *c.GlobalValuesName, rowID); err != nil {
			return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
		}
	}
	return 0, "", ""
}

// markPRMerged flips the PR to merged and refreshes pr from the returned row.
func markPRMerged(ctx context.Context, tx *sql.Tx, prID int64, pr *models.PullRequest) error {
	return tx.QueryRowContext(ctx, `
		UPDATE pull_requests
		SET status = 'merged', merged_at = NOW(), updated_at = NOW()
		WHERE id = $1
		RETURNING id, project_id, global_values_name, base_project_version_id, base_group_version_id,
		          author_id, title, description, status, is_conflicted,
		          created_at, updated_at, merged_at, closed_at
	`, prID).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.BaseProjectVersionID, &pr.BaseGroupVersionID,
		&pr.AuthorID, &pr.Title, &pr.Description, &pr.Status, &pr.IsConflicted,
		&pr.CreatedAt, &pr.UpdatedAt, &pr.MergedAt, &pr.ClosedAt,
	)
}

// autoCloseSiblingPRs closes any other unmerged PR/workspace targeting the same
// scope (project or global-values name) so the just-merged PR's changes become
// canonical and stale workspaces are invalidated.
func autoCloseSiblingPRs(ctx context.Context, tx *sql.Tx, pr models.PullRequest, prID int64) error {
	switch {
	case pr.GlobalValuesName != nil:
		_, err := tx.ExecContext(ctx, `
			UPDATE pull_requests
			SET status = 'closed', closed_at = NOW(), updated_at = NOW()
			WHERE global_values_name = $1
			  AND id != $2
			  AND status IN ('draft', 'open', 'approved')
		`, *pr.GlobalValuesName, prID)
		return err
	case pr.ProjectID != nil:
		_, err := tx.ExecContext(ctx, `
			UPDATE pull_requests
			SET status = 'closed', closed_at = NOW(), updated_at = NOW()
			WHERE project_id = $1
			  AND id != $2
			  AND status IN ('draft', 'open', 'approved')
		`, *pr.ProjectID, prID)
		return err
	}
	return nil
}

func (h *PullRequestHandler) Merge(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	prID, err := urlParamInt64(r, "prID")
	if err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidPRID, "bad_request")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tx.Rollback()

	pr, status, msg, code := loadAndLockPRForMerge(r.Context(), tx, prID, user.UserID)
	if status != 0 {
		writeError(w, status, msg, code)
		return
	}

	changes, err := loadPRChangesTx(r.Context(), tx, prID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	commitMsg := fmt.Sprintf("Merged from PR #%d", pr.ID)
	if status, msg, code := applyPRChanges(r.Context(), tx, &pr, changes, pr.AuthorID, commitMsg); status != 0 {
		writeError(w, status, msg, code)
		return
	}

	if err := markPRMerged(r.Context(), tx, prID, &pr); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update pull request", "internal")
		return
	}

	if err := autoCloseSiblingPRs(r.Context(), tx, pr, prID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to auto-close other PRs", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, msgFailedToCommit, "internal")
		return
	}

	pr.Changes = changes
	writeJSON(w, http.StatusOK, pr)
}

// GetActiveDraft returns the user's active (draft/open/approved) PR for a project,
// or creates a new draft if none exists.
func (h *PullRequestHandler) GetActiveDraft(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projectName := chi.URLParam(r, "projectName")

	// Resolve project ID.
	var projectID int64
	err := h.DB.QueryRowContext(r.Context(), `SELECT id FROM projects WHERE name = $1`, projectName).Scan(&projectID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	// Look for an existing active PR.
	var pr models.PullRequest
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, project_id, global_values_name, author_id, title, description, status,
		       is_conflicted, created_at, updated_at, merged_at, closed_at
		FROM pull_requests
		WHERE project_id = $1 AND author_id = $2 AND status IN ('draft', 'open', 'approved')
		ORDER BY created_at DESC LIMIT 1
	`, projectID, user.UserID).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
	)
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	if err == sql.ErrNoRows {
		// Create a new draft.
		err = h.DB.QueryRowContext(r.Context(), `
			INSERT INTO pull_requests (project_id, author_id, title, description, status)
			VALUES ($1, $2, '', NULL, 'draft')
			RETURNING id, project_id, global_values_name, author_id, title, description, status,
			          is_conflicted, created_at, updated_at, merged_at, closed_at
		`, projectID, user.UserID).Scan(
			&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
			&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
			&pr.MergedAt, &pr.ClosedAt,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create draft", "internal")
			return
		}
	}

	// Load changes.
	pr.Changes, err = h.loadChanges(r.Context(), pr.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	// Load approval condition.
	condition, err := h.loadApprovalCondition(r.Context(), &pr)
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	pr.ApprovalCondition = condition

	// Load approvals.
	pr.Approvals, err = h.loadApprovals(r.Context(), pr.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

// SubmitDraft transitions a draft PR to open with a title and description.
func (h *PullRequestHandler) SubmitDraft(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	prID, err := urlParamInt64(r, "prID")
	if err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidPRID, "bad_request")
		return
	}

	var req models.SubmitDraftRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required", "validation")
		return
	}

	var pr models.PullRequest
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, project_id, global_values_name, author_id, title, description, status,
		       is_conflicted, created_at, updated_at, merged_at, closed_at
		FROM pull_requests WHERE id = $1
	`, prID).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgPullRequestNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	if pr.AuthorID != user.UserID {
		writeError(w, http.StatusForbidden, "only the author can submit a draft", "forbidden")
		return
	}
	if pr.Status != "draft" {
		writeError(w, http.StatusConflict, "pull request is not a draft", "conflict")
		return
	}

	// Block submitting a project workspace whose staged config cannot render
	// (e.g. a template was created but no environment supplies its values).
	if pr.ProjectID != nil {
		problems, verr := h.validateWorkspace(r.Context(), *pr.ProjectID, user.UserID)
		if verr != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		if len(problems) > 0 {
			writeJSON(w, http.StatusUnprocessableEntity, struct {
				Error    string                    `json:"error"`
				Code     string                    `json:"code"`
				Problems []models.WorkspaceProblem `json:"problems"`
			}{
				Error:    fmt.Sprintf("workspace has %d problem(s); resolve them before submitting", len(problems)),
				Code:     "workspace_invalid",
				Problems: problems,
			})
			return
		}
	}

	err = h.DB.QueryRowContext(r.Context(), `
		UPDATE pull_requests SET title = $2, description = $3, status = 'open', updated_at = NOW()
		WHERE id = $1
		RETURNING id, project_id, global_values_name, author_id, title, description, status,
		          is_conflicted, created_at, updated_at, merged_at, closed_at
	`, prID, req.Title, req.Description).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to submit draft", "internal")
		return
	}

	pr.Changes, _ = h.loadChanges(r.Context(), pr.ID)
	writeJSON(w, http.StatusOK, pr)
}

// loadChanges fetches all changes for a PR.
func (h *PullRequestHandler) loadChanges(ctx context.Context, prID int64) ([]models.PRChange, error) {
	rows, err := h.DB.QueryContext(ctx, `
		SELECT id, pr_id, object_type, operation, project_id, template_name,
		       environment_name, global_values_name, proposed_payload, created_at
		FROM pr_changes WHERE pr_id = $1
		ORDER BY id
	`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	changes := []models.PRChange{}
	for rows.Next() {
		var c models.PRChange
		if err := rows.Scan(
			&c.ID, &c.PRID, &c.ObjectType, &c.Operation, &c.ProjectID, &c.TemplateName,
			&c.EnvironmentName, &c.GlobalValuesName, &c.ProposedPayload, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		changes = append(changes, c)
	}
	return changes, rows.Err()
}

func (h *PullRequestHandler) List(w http.ResponseWriter, r *http.Request) {
	gvName := r.URL.Query().Get("global_values_name")

	var (
		rows *sql.Rows
		err  error
	)

	if gvName != "" {
		rows, err = h.DB.QueryContext(r.Context(), `
			SELECT p.id, p.project_id, p.global_values_name, p.author_id, p.title, p.description,
			       p.status, p.is_conflicted, p.created_at, p.updated_at,
			       p.merged_at, p.closed_at
			FROM pull_requests p
			WHERE p.global_values_name = $1
			ORDER BY p.created_at DESC
		`, gvName)
	} else {
		rows, err = h.DB.QueryContext(r.Context(), `
			SELECT id, project_id, global_values_name, author_id, title, description, status,
			       is_conflicted, created_at, updated_at, merged_at, closed_at
			FROM pull_requests
			ORDER BY created_at DESC
		`)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer rows.Close()

	items := []models.PullRequest{}
	for rows.Next() {
		var pr models.PullRequest
		if err := rows.Scan(
			&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
			&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
			&pr.MergedAt, &pr.ClosedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		items = append(items, pr)
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.PullRequest]{Items: items, Count: len(items)})
}
