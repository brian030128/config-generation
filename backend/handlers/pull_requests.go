package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/brian/config-generation/backend/models"
)

type PullRequestHandler struct {
	DB *sql.DB
}

func (h *PullRequestHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	var req models.CreatePullRequestRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
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
		writeError(w, http.StatusInternalServerError, "database error", "internal")
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
		writeError(w, http.StatusInternalServerError, "database error", "internal")
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
		INSERT INTO pr_changes (pr_id, object_type, global_values_name, base_version_id, proposed_payload)
		VALUES ($1, 'global_values', $2, $3, $4)
		RETURNING id, pr_id, object_type, project_id, template_name,
		          environment_id, global_values_name, base_version_id,
		          proposed_payload, created_at
	`, pr.ID, *req.GlobalValuesName, baseVersionID, req.ProposedPayload).Scan(
		&change.ID, &change.PRID, &change.ObjectType, &change.ProjectID,
		&change.TemplateName, &change.EnvironmentID, &change.GlobalValuesName,
		&change.BaseVersionID, &change.ProposedPayload, &change.CreatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create pr change", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
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

// checkApprovalConditionMet checks if the approval condition is satisfied
// by the current approvals. It queries role membership for each approver.
func (h *PullRequestHandler) checkApprovalConditionMet(ctx context.Context, pr *models.PullRequest, condition string, approvals []models.PRApproval) (bool, error) {
	reqs := parseApprovalCondition(condition)
	if len(reqs) == 0 {
		return true, nil
	}

	// Determine the scope for roles.
	var scopeFilter string
	var scopeArgs []any

	if pr.GlobalValuesName != nil {
		scopeFilter = "r.global_values_name = $1"
		scopeArgs = append(scopeArgs, *pr.GlobalValuesName)
	} else if pr.ProjectID != nil {
		scopeFilter = "r.project_id = $1"
		scopeArgs = append(scopeArgs, *pr.ProjectID)
	} else {
		return false, nil
	}

	// For each approver, find their roles.
	approverRoles := make(map[int64][]string)
	for _, a := range approvals {
		rows, err := h.DB.QueryContext(ctx,
			`SELECT r.name FROM roles r
			 JOIN user_roles ur ON ur.role_id = r.id
			 WHERE ur.user_id = $2 AND `+scopeFilter,
			scopeArgs[0], a.UserID)
		if err != nil {
			return false, err
		}
		var roles []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return false, err
			}
			roles = append(roles, name)
		}
		rows.Close()
		approverRoles[a.UserID] = roles
	}

	// Check if each requirement is satisfied.
	// Use AND semantics: all requirements must be met.
	isAnd := strings.Contains(strings.ToUpper(condition), "AND") || len(reqs) == 1

	if isAnd {
		for _, req := range reqs {
			count := 0
			for _, roles := range approverRoles {
				for _, r := range roles {
					// Match role name: the role name in the condition may be just the
					// base name (e.g., "gv_group_admin"), but the actual role name
					// includes the scope suffix (e.g., "gv_group_admin:test_db_values").
					if r == req.RoleName || strings.HasPrefix(r, req.RoleName+":") {
						count++
						break
					}
				}
			}
			if count < req.Count {
				return false, nil
			}
		}
		return true, nil
	}

	// OR semantics: at least one requirement must be met.
	for _, req := range reqs {
		count := 0
		for _, roles := range approverRoles {
			for _, r := range roles {
				if r == req.RoleName || strings.HasPrefix(r, req.RoleName+":") {
					count++
					break
				}
			}
		}
		if count >= req.Count {
			return true, nil
		}
	}
	return false, nil
}

func (h *PullRequestHandler) Approve(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	prID, err := urlParamInt64(r, "prID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PR ID", "bad_request")
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
		writeError(w, http.StatusNotFound, "pull request not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
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

func (h *PullRequestHandler) WithdrawApproval(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	prID, err := urlParamInt64(r, "prID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PR ID", "bad_request")
		return
	}

	result, err := h.DB.ExecContext(r.Context(), `
		UPDATE pr_approvals SET withdrawn_at = NOW()
		WHERE pr_id = $1 AND user_id = $2 AND withdrawn_at IS NULL
	`, prID, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
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
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	if pr.Status == "approved" {
		condition, err := h.loadApprovalCondition(r.Context(), &pr)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		approvals, err := h.loadApprovals(r.Context(), prID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		met, err := h.checkApprovalConditionMet(r.Context(), &pr, condition, approvals)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		if !met {
			h.DB.ExecContext(r.Context(), `
				UPDATE pull_requests SET status = 'open', updated_at = NOW() WHERE id = $1
			`, prID)
			pr.Status = "open"
		}
		pr.ApprovalCondition = condition
		pr.Approvals = approvals
	}

	writeJSON(w, http.StatusOK, pr)
}

func (h *PullRequestHandler) Get(w http.ResponseWriter, r *http.Request) {
	prID, err := urlParamInt64(r, "prID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PR ID", "bad_request")
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
		writeError(w, http.StatusNotFound, "pull request not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, pr_id, object_type, project_id, template_name,
		       environment_id, global_values_name, base_version_id,
		       proposed_payload, created_at
		FROM pr_changes WHERE pr_id = $1
		ORDER BY id
	`, prID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer rows.Close()

	pr.Changes = []models.PRChange{}
	for rows.Next() {
		var c models.PRChange
		if err := rows.Scan(
			&c.ID, &c.PRID, &c.ObjectType, &c.ProjectID, &c.TemplateName,
			&c.EnvironmentID, &c.GlobalValuesName, &c.BaseVersionID,
			&c.ProposedPayload, &c.CreatedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		pr.Changes = append(pr.Changes, c)
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
		writeError(w, http.StatusBadRequest, "invalid PR ID", "bad_request")
		return
	}

	var currentStatus string
	err = h.DB.QueryRowContext(r.Context(),
		`SELECT status FROM pull_requests WHERE id = $1`, prID,
	).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "pull request not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
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

func (h *PullRequestHandler) Merge(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	prID, err := urlParamInt64(r, "prID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PR ID", "bad_request")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer tx.Rollback()

	// Load the PR with a row lock.
	var pr models.PullRequest
	err = tx.QueryRowContext(r.Context(), `
		SELECT id, project_id, global_values_name, author_id, title, description, status,
		       is_conflicted, created_at, updated_at, merged_at, closed_at
		FROM pull_requests WHERE id = $1 FOR UPDATE
	`, prID).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "pull request not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	// Only the author can merge.
	if pr.AuthorID != user.UserID {
		writeError(w, http.StatusForbidden, "only the PR author can merge", "forbidden")
		return
	}

	// PR must be approved.
	if pr.Status != "approved" {
		writeError(w, http.StatusConflict, "pull request must be approved before merging", "conflict")
		return
	}

	if pr.IsConflicted {
		writeError(w, http.StatusConflict, "pull request has conflicts and cannot be merged", "conflict")
		return
	}

	// Only global values PRs are supported for now.
	if pr.GlobalValuesName == nil {
		writeError(w, http.StatusBadRequest, "only global values PRs can be merged at this time", "validation")
		return
	}

	// Load changes.
	changeRows, err := tx.QueryContext(r.Context(), `
		SELECT id, pr_id, object_type, project_id, template_name,
		       environment_id, global_values_name, base_version_id,
		       proposed_payload, created_at
		FROM pr_changes WHERE pr_id = $1
	`, prID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer changeRows.Close()

	var changes []models.PRChange
	for changeRows.Next() {
		var c models.PRChange
		if err := changeRows.Scan(
			&c.ID, &c.PRID, &c.ObjectType, &c.ProjectID, &c.TemplateName,
			&c.EnvironmentID, &c.GlobalValuesName, &c.BaseVersionID,
			&c.ProposedPayload, &c.CreatedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		changes = append(changes, c)
	}
	changeRows.Close()

	// Apply each change: append a new version for the global values entry.
	commitMsg := fmt.Sprintf("Merged from PR #%d", pr.ID)
	for _, c := range changes {
		if c.ObjectType != "global_values" || c.GlobalValuesName == nil {
			continue
		}

		// Get the next version ID.
		var nextVersion int
		err = tx.QueryRowContext(r.Context(), `
			SELECT COALESCE(
				(SELECT version_id FROM global_values
				 WHERE name = $1
				 ORDER BY version_id DESC LIMIT 1
				 FOR UPDATE),
			0) + 1
		`, *c.GlobalValuesName).Scan(&nextVersion)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}

		// Insert the new version.
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO global_values (name, version_id, payload, commit_message, approval_condition, created_by)
			VALUES ($1, $2, $3, $4,
				(SELECT approval_condition FROM global_values WHERE name = $1 ORDER BY version_id LIMIT 1),
				$5)
		`, *c.GlobalValuesName, nextVersion, c.ProposedPayload, commitMsg, pr.AuthorID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create new version", "internal")
			return
		}
	}

	// Mark the PR as merged.
	err = tx.QueryRowContext(r.Context(), `
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
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update pull request", "internal")
		return
	}

	// Auto-close all other unmerged PRs on the same global values entry.
	_, err = tx.ExecContext(r.Context(), `
		UPDATE pull_requests
		SET status = 'closed', closed_at = NOW(), updated_at = NOW()
		WHERE global_values_name = $1
		  AND id != $2
		  AND status IN ('draft', 'open', 'approved')
	`, *pr.GlobalValuesName, prID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to auto-close other PRs", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	pr.Changes = changes
	writeJSON(w, http.StatusOK, pr)
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
		writeError(w, http.StatusInternalServerError, "database error", "internal")
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
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		items = append(items, pr)
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.PullRequest]{Items: items, Count: len(items)})
}
