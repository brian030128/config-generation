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

	// Look up the current latest version_id for the global values entry.
	var baseVersionID int
	err = tx.QueryRowContext(r.Context(), `
		SELECT COALESCE(
			(SELECT version_id FROM global_values
			 WHERE name = $1
			 ORDER BY version_id DESC LIMIT 1),
		0)
	`, *req.GlobalValuesName).Scan(&baseVersionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	if baseVersionID == 0 {
		writeError(w, http.StatusNotFound, "global values entry not found", "not_found")
		return
	}

	// Create the pull request row.
	var pr models.PullRequest
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO pull_requests (project_id, global_values_name, author_id, title, description, status)
		VALUES (NULL, $1, $2, $3, $4, 'open')
		RETURNING id, project_id, global_values_name, author_id, title, description, status,
		          is_conflicted, created_at, updated_at, merged_at, closed_at
	`, *req.GlobalValuesName, user.UserID, req.Title, req.Description).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create pull request", "internal")
		return
	}

	// Create the pr_changes row.
	var change models.PRChange
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO pr_changes (pr_id, object_type, operation, global_values_name, base_version_id, proposed_payload)
		VALUES ($1, 'global_values', 'update', $2, $3, $4)
		RETURNING id, pr_id, object_type, operation, project_id, template_name,
		          environment_name, global_values_name, base_version_id,
		          proposed_payload, created_at
	`, pr.ID, *req.GlobalValuesName, baseVersionID, req.ProposedPayload).Scan(
		&change.ID, &change.PRID, &change.ObjectType, &change.Operation, &change.ProjectID,
		&change.TemplateName, &change.EnvironmentName, &change.GlobalValuesName,
		&change.BaseVersionID, &change.ProposedPayload, &change.CreatedAt,
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
			SELECT approval_condition FROM global_values
			WHERE name = $1 ORDER BY version_id LIMIT 1
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

// requirementsMet reports whether the set of approverRoles satisfies all
// requirements (AND) or any one of them (OR).
func requirementsMet(reqs []roleRequirement, approverRoles map[int64][]string, isAnd bool) bool {
	countApprovers := func(reqName string) int {
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
	if isAnd {
		for _, req := range reqs {
			if countApprovers(req.RoleName) < req.Count {
				return false
			}
		}
		return true
	}
	for _, req := range reqs {
		if countApprovers(req.RoleName) >= req.Count {
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

// prMergeOrderedTypes is the apply order; environments come first so values
// can FK to them, and templates before values because values may refer to them.
var prMergeOrderedTypes = []string{"environment", "global_values", "template", "values"}

// loadAndLockPRForMerge fetches the PR with a row lock and validates that it
// is eligible to be merged by the given user.
func loadAndLockPRForMerge(ctx context.Context, tx *sql.Tx, prID, userID int64) (models.PullRequest, int, string, string) {
	var pr models.PullRequest
	err := tx.QueryRowContext(ctx, `
		SELECT id, project_id, global_values_name, author_id, title, description, status,
		       is_conflicted, created_at, updated_at, merged_at, closed_at
		FROM pull_requests WHERE id = $1 FOR UPDATE
	`, prID).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
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
		       environment_name, global_values_name, base_version_id,
		       proposed_payload, created_at
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
			&c.EnvironmentName, &c.GlobalValuesName, &c.BaseVersionID,
			&c.ProposedPayload, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		changes = append(changes, c)
	}
	return changes, rows.Err()
}

// applyPRChanges replays each PR change against the live tables, in the
// type-ordered passes required by FK dependencies.
func applyPRChanges(ctx context.Context, tx *sql.Tx, changes []models.PRChange, authorID int64, commitMsg string) (int, string, string) {
	for _, objType := range prMergeOrderedTypes {
		for _, c := range changes {
			if c.ObjectType != objType {
				continue
			}
			if status, msg, code := applyPRChange(ctx, tx, c, authorID, commitMsg); status != 0 {
				return status, msg, code
			}
		}
	}
	return 0, "", ""
}

// applyPRChange dispatches a single change to the per-type apply function.
func applyPRChange(ctx context.Context, tx *sql.Tx, c models.PRChange, authorID int64, commitMsg string) (int, string, string) {
	var err error
	switch c.ObjectType {
	case "environment":
		err = applyEnvironmentChange(ctx, tx, c, authorID)
	case "global_values":
		err = applyGlobalValuesChange(ctx, tx, c, authorID, commitMsg)
	case "template":
		err = applyTemplateChange(ctx, tx, c, authorID, commitMsg)
	case "values":
		return applyValuesChange(ctx, tx, c, authorID, commitMsg)
	}
	if err != nil {
		return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
	}
	return 0, "", ""
}

func applyEnvironmentChange(ctx context.Context, tx *sql.Tx, c models.PRChange, authorID int64) error {
	if c.ProjectID == nil || c.EnvironmentName == nil {
		return nil
	}
	if c.Operation == "delete" {
		// Tear down the env's value sets and env-admin grants first (FK), then
		// the env itself. The environment id is resolved inside a single SQL
		// literal (no concatenation) using a parameterised subquery.
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM project_config_values
			WHERE project_id = $1
			  AND environment_id = (
				SELECT id FROM environments WHERE project_id = $1 AND name = $2
			)`,
			*c.ProjectID, *c.EnvironmentName); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM env_admins WHERE environment_id = (
				SELECT id FROM environments WHERE project_id = $1 AND name = $2
			)`,
			*c.ProjectID, *c.EnvironmentName); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			DELETE FROM environments WHERE project_id = $1 AND name = $2
		`, *c.ProjectID, *c.EnvironmentName)
		return err
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
	// Seed the creator as the environment's first env-admin. Resolving the id
	// via SELECT handles the ON CONFLICT DO NOTHING case above; the env_admins
	// upsert is then a no-op too.
	_, err := tx.ExecContext(ctx, `
		INSERT INTO env_admins (environment_id, user_id, granted_by)
		SELECT id, $3, $3 FROM environments WHERE project_id = $1 AND name = $2
		ON CONFLICT (environment_id, user_id) DO NOTHING
	`, *c.ProjectID, envReq.Name, authorID)
	return err
}

func applyGlobalValuesChange(ctx context.Context, tx *sql.Tx, c models.PRChange, authorID int64, commitMsg string) error {
	if c.GlobalValuesName == nil {
		return nil
	}
	var nextVersion int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(
			(SELECT version_id FROM global_values
			 WHERE name = $1 ORDER BY version_id DESC LIMIT 1 FOR UPDATE),
		0) + 1
	`, *c.GlobalValuesName).Scan(&nextVersion); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO global_values (name, version_id, payload, commit_message, approval_condition, created_by)
		VALUES ($1, $2, $3, $4,
			(SELECT approval_condition FROM global_values WHERE name = $1 ORDER BY version_id LIMIT 1),
			$5)
	`, *c.GlobalValuesName, nextVersion, c.ProposedPayload, commitMsg, authorID)
	return err
}

func applyTemplateChange(ctx context.Context, tx *sql.Tx, c models.PRChange, authorID int64, commitMsg string) error {
	if c.ProjectID == nil || c.TemplateName == nil {
		return nil
	}
	if c.Operation == "delete" {
		_, err := tx.ExecContext(ctx, `
			DELETE FROM project_config_templates WHERE project_id = $1 AND template_name = $2
		`, *c.ProjectID, *c.TemplateName)
		return err
	}
	var nextVersion int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(
			(SELECT version_id FROM project_config_templates
			 WHERE project_id = $1 AND template_name = $2
			 ORDER BY version_id DESC LIMIT 1 FOR UPDATE),
		0) + 1
	`, *c.ProjectID, *c.TemplateName).Scan(&nextVersion); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO project_config_templates (project_id, template_name, version_id, body, commit_message, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, *c.ProjectID, *c.TemplateName, nextVersion, c.ProposedPayload, commitMsg, authorID)
	return err
}

// applyValuesChange returns an HTTP-shaped error because a missing environment
// during a non-delete values change is a 500 with a distinct message.
func applyValuesChange(ctx context.Context, tx *sql.Tx, c models.PRChange, authorID int64, commitMsg string) (int, string, string) {
	if c.ProjectID == nil || c.EnvironmentName == nil {
		return 0, "", ""
	}
	var envID int64
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM environments WHERE project_id = $1 AND name = $2
	`, *c.ProjectID, *c.EnvironmentName).Scan(&envID)
	if err == sql.ErrNoRows {
		// If the env is gone (e.g. deleted in the same PR), a values delete is
		// a no-op; anything else is an error.
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
			DELETE FROM project_config_values WHERE project_id = $1 AND environment_id = $2
		`, *c.ProjectID, envID); err != nil {
			return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
		}
		return 0, "", ""
	}
	var nextVersion int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(
			(SELECT version_id FROM project_config_values
			 WHERE project_id = $1 AND environment_id = $2
			 ORDER BY version_id DESC LIMIT 1 FOR UPDATE),
		0) + 1
	`, *c.ProjectID, envID).Scan(&nextVersion); err != nil {
		return http.StatusInternalServerError, msgDatabaseError, "internal"
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_config_values (project_id, environment_id, version_id, payload, commit_message, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, *c.ProjectID, envID, nextVersion, c.ProposedPayload, commitMsg, authorID); err != nil {
		return http.StatusInternalServerError, msgFailedToApplyChange, "internal"
	}
	return 0, "", ""
}

// markPRMerged flips the PR to merged and refreshes pr from the returned row.
func markPRMerged(ctx context.Context, tx *sql.Tx, prID int64, pr *models.PullRequest) error {
	return tx.QueryRowContext(ctx, `
		UPDATE pull_requests
		SET status = 'merged', merged_at = NOW(), updated_at = NOW()
		WHERE id = $1
		RETURNING id, project_id, global_values_name, author_id, title, description, status,
		          is_conflicted, created_at, updated_at, merged_at, closed_at
	`, prID).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
	)
}

// autoCloseSiblingPRs closes any other unmerged PR targeting the same global
// values scope so the just-merged PR's changes become canonical.
func autoCloseSiblingPRs(ctx context.Context, tx *sql.Tx, pr models.PullRequest, prID int64) error {
	if pr.GlobalValuesName == nil {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE pull_requests
		SET status = 'closed', closed_at = NOW(), updated_at = NOW()
		WHERE global_values_name = $1
		  AND id != $2
		  AND status IN ('draft', 'open', 'approved')
	`, *pr.GlobalValuesName, prID)
	return err
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
	if status, msg, code := applyPRChanges(r.Context(), tx, changes, pr.AuthorID, commitMsg); status != 0 {
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
		       environment_name, global_values_name, base_version_id,
		       proposed_payload, created_at
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
			&c.EnvironmentName, &c.GlobalValuesName, &c.BaseVersionID,
			&c.ProposedPayload, &c.CreatedAt,
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
