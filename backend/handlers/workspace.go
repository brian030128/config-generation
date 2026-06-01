package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/brian/config-generation/backend/middleware"
	"github.com/brian/config-generation/backend/models"
	"github.com/go-chi/chi/v5"
)

// The workspace is the project write surface: the caller's single active
// (draft/open/approved) Project PR. Every project mutation is staged here and
// only changes live state when the PR is merged. There is one endpoint per
// object type and operation so OpenAPI codegen produces distinct models.

// stagedChange is one change to upsert into the caller's workspace.
type stagedChange struct {
	objectType   string // "template" | "values" | "environment"
	operation    string // "create" | "update" | "delete"
	templateName *string
	envName      *string
	payload      string // "" for delete
}

// findOrCreateActiveDraft returns the caller's active PR for the project,
// creating an empty draft if none exists. The row is locked FOR UPDATE.
func findOrCreateActiveDraft(ctx context.Context, tx *sql.Tx, projectID, userID int64) (models.PullRequest, error) {
	var pr models.PullRequest
	err := tx.QueryRowContext(ctx, `
		SELECT id, project_id, global_values_name, author_id, title, description, status,
		       is_conflicted, created_at, updated_at, merged_at, closed_at
		FROM pull_requests
		WHERE project_id = $1 AND author_id = $2 AND status IN ('draft', 'open', 'approved')
		ORDER BY created_at DESC LIMIT 1
		FOR UPDATE
	`, projectID, userID).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt, &pr.MergedAt, &pr.ClosedAt,
	)
	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO pull_requests (project_id, author_id, title, description, status)
			VALUES ($1, $2, '', NULL, 'draft')
			RETURNING id, project_id, global_values_name, author_id, title, description, status,
			          is_conflicted, created_at, updated_at, merged_at, closed_at
		`, projectID, userID).Scan(
			&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
			&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt, &pr.MergedAt, &pr.ClosedAt,
		)
	}
	return pr, err
}

// activeDraftID returns the caller's active PR id for the project without
// creating one. ok is false when no active workspace exists.
func (h *PullRequestHandler) activeDraftID(ctx context.Context, projectID, userID int64) (int64, bool, error) {
	var id int64
	err := h.DB.QueryRowContext(ctx, `
		SELECT id FROM pull_requests
		WHERE project_id = $1 AND author_id = $2 AND status IN ('draft', 'open', 'approved')
		ORDER BY created_at DESC LIMIT 1
	`, projectID, userID).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// resolveProjectIDByName looks up a project id by name (workspace routes use
// {projectName}). Returns ErrNoRows if absent.
func (h *PullRequestHandler) resolveProjectIDByName(ctx context.Context, name string) (int64, error) {
	var id int64
	err := h.DB.QueryRowContext(ctx, `SELECT id FROM projects WHERE name = $1`, name).Scan(&id)
	return id, err
}

// stage upserts a single change into the caller's workspace and returns the
// refreshed PR. Staging into an approved PR resets approvals and reopens it.
func (h *PullRequestHandler) stage(w http.ResponseWriter, r *http.Request, sc stagedChange) {
	user := currentUser(r)
	projectName := chi.URLParam(r, "projectName")

	projectID, err := h.resolveProjectIDByName(r.Context(), projectName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tx.Rollback()

	pr, err := findOrCreateActiveDraft(r.Context(), tx, projectID, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open workspace", "internal")
		return
	}

	if pr.Status == "approved" {
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE pr_approvals SET withdrawn_at = NOW() WHERE pr_id = $1 AND withdrawn_at IS NULL
		`, pr.ID); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE pull_requests SET status = 'open', updated_at = NOW() WHERE id = $1
		`, pr.ID); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		pr.Status = "open"
	}

	// Capture the base version for conflict detection on versioned objects.
	var baseVersionID int
	switch sc.objectType {
	case "template":
		if err = tx.QueryRowContext(r.Context(), `
			SELECT COALESCE(
				(SELECT version_id FROM project_config_templates
				 WHERE project_id = $1 AND template_name = $2
				 ORDER BY version_id DESC LIMIT 1),
			0)
		`, projectID, sc.templateName).Scan(&baseVersionID); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
	case "values":
		var envID int64
		if tx.QueryRowContext(r.Context(),
			`SELECT id FROM environments WHERE project_id = $1 AND name = $2`,
			projectID, sc.envName).Scan(&envID) == nil {
			_ = tx.QueryRowContext(r.Context(), `
				SELECT COALESCE(
					(SELECT version_id FROM project_config_values
					 WHERE project_id = $1 AND environment_id = $2
					 ORDER BY version_id DESC LIMIT 1),
				0)
			`, projectID, envID).Scan(&baseVersionID)
		}
	}

	// Replace any prior staged change for the same object (last write wins).
	switch sc.objectType {
	case "template":
		_, err = tx.ExecContext(r.Context(),
			`DELETE FROM pr_changes WHERE pr_id = $1 AND object_type = 'template' AND template_name = $2`,
			pr.ID, sc.templateName)
	case "values":
		_, err = tx.ExecContext(r.Context(),
			`DELETE FROM pr_changes WHERE pr_id = $1 AND object_type = 'values' AND environment_name = $2`,
			pr.ID, sc.envName)
	case "environment":
		_, err = tx.ExecContext(r.Context(),
			`DELETE FROM pr_changes WHERE pr_id = $1 AND object_type = 'environment' AND environment_name = $2`,
			pr.ID, sc.envName)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO pr_changes (pr_id, object_type, operation, project_id, template_name, environment_name, base_version_id, proposed_payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, pr.ID, sc.objectType, sc.operation, projectID, sc.templateName, sc.envName, baseVersionID, sc.payload); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to stage change", "internal")
		return
	}

	if _, err = tx.ExecContext(r.Context(), `UPDATE pull_requests SET updated_at = NOW() WHERE id = $1`, pr.ID); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	if err = tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, msgFailedToCommit, "internal")
		return
	}

	pr.Changes, _ = h.loadChanges(r.Context(), pr.ID)
	writeJSON(w, http.StatusOK, pr)
}

// ── Templates ──────────────────────────────────────────────────────────────

func (h *PullRequestHandler) StageTemplateCreate(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if req.TemplateName == "" || req.Body == "" {
		writeError(w, http.StatusBadRequest, "template_name and body are required", "validation")
		return
	}
	name := req.TemplateName
	h.stage(w, r, stagedChange{objectType: "template", operation: "create", templateName: &name, payload: req.Body})
}

func (h *PullRequestHandler) StageTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "templateName")
	var req models.AppendTemplateVersionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if req.Body == "" {
		writeError(w, http.StatusBadRequest, "body is required", "validation")
		return
	}
	h.stage(w, r, stagedChange{objectType: "template", operation: "update", templateName: &name, payload: req.Body})
}

func (h *PullRequestHandler) StageTemplateDelete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "templateName")
	h.stage(w, r, stagedChange{objectType: "template", operation: "delete", templateName: &name})
}

// ── Environments ─────────────────────────────────────────────────────────────

func (h *PullRequestHandler) StageEnvironmentCreate(w http.ResponseWriter, r *http.Request) {
	var req models.CreateEnvironmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required", "validation")
		return
	}
	payload, _ := json.Marshal(req)
	name := req.Name
	h.stage(w, r, stagedChange{objectType: "environment", operation: "create", envName: &name, payload: string(payload)})
}

func (h *PullRequestHandler) StageEnvironmentDelete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "envName")
	h.stage(w, r, stagedChange{objectType: "environment", operation: "delete", envName: &name})
}

// ── Values ───────────────────────────────────────────────────────────────────

func (h *PullRequestHandler) StageValuesUpsert(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projectName := chi.URLParam(r, "projectName")
	envName := chi.URLParam(r, "envName")

	var req models.AppendProjectConfigValuesVersionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if len(req.Payload) == 0 {
		writeError(w, http.StatusBadRequest, "payload is required", "validation")
		return
	}

	projectID, err := h.resolveProjectIDByName(r.Context(), projectName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	// Creating the first (v1) value set for an env requires create:env_values;
	// editing an existing set only needs write (gated on the route).
	var liveExists bool
	if err = h.DB.QueryRowContext(r.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM project_config_values v
			JOIN environments e ON e.id = v.environment_id
			WHERE v.project_id = $1 AND e.name = $2
		)
	`, projectID, envName).Scan(&liveExists); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	operation := "update"
	if !liveExists {
		operation = "create"
		ok, err := middleware.CheckPermission(r.Context(), h.DB, user.UserID, models.PermissionRequirement{
			Action:     models.ActionCreate,
			Resource:   models.ResourceEnvValues,
			KeyProject: projectName,
			KeyEnv:     envName,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check permissions", "internal")
			return
		}
		if !ok {
			writeError(w, http.StatusForbidden, "create:env_values required to create a new value set", "forbidden")
			return
		}
	}

	h.stage(w, r, stagedChange{objectType: "values", operation: operation, envName: &envName, payload: string(req.Payload)})
}

func (h *PullRequestHandler) StageValuesDelete(w http.ResponseWriter, r *http.Request) {
	envName := chi.URLParam(r, "envName")
	h.stage(w, r, stagedChange{objectType: "values", operation: "delete", envName: &envName})
}

// ── Lifecycle / change set ─────────────────────────────────────────────────

// DiscardWorkspace deletes the caller's draft workspace and its staged changes.
// Only a draft can be discarded; submitted PRs must be closed via the PR API.
func (h *PullRequestHandler) DiscardWorkspace(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projectName := chi.URLParam(r, "projectName")

	projectID, err := h.resolveProjectIDByName(r.Context(), projectName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tx.Rollback()

	var prID int64
	var status string
	err = tx.QueryRowContext(r.Context(), `
		SELECT id, status FROM pull_requests
		WHERE project_id = $1 AND author_id = $2 AND status IN ('draft', 'open', 'approved')
		ORDER BY created_at DESC LIMIT 1
		FOR UPDATE
	`, projectID, user.UserID).Scan(&prID, &status)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "no active workspace", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	if status != "draft" {
		writeError(w, http.StatusConflict, "workspace already submitted; close the pull request instead", "conflict")
		return
	}

	for _, q := range []string{
		`DELETE FROM pr_approvals WHERE pr_id = $1`,
		`DELETE FROM pr_changes WHERE pr_id = $1`,
		`DELETE FROM pull_requests WHERE id = $1`,
	} {
		if _, err = tx.ExecContext(r.Context(), q, prID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to discard workspace", "internal")
			return
		}
	}

	if err = tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, msgFailedToCommit, "internal")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListChanges returns the staged changes in the caller's active workspace.
func (h *PullRequestHandler) ListChanges(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projectName := chi.URLParam(r, "projectName")

	projectID, err := h.resolveProjectIDByName(r.Context(), projectName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	prID, ok, err := h.activeDraftID(r.Context(), projectID, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	changes := []models.PRChange{}
	if ok {
		if changes, err = h.loadChanges(r.Context(), prID); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.PRChange]{Items: changes, Count: len(changes)})
}

// UnstageChange removes one staged change, reverting that object to the
// published base within the workspace. Unstaging from an approved PR resets
// approvals and reopens it.
func (h *PullRequestHandler) UnstageChange(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projectName := chi.URLParam(r, "projectName")
	changeID, err := urlParamInt64(r, "changeID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid change ID", "bad_request")
		return
	}

	projectID, err := h.resolveProjectIDByName(r.Context(), projectName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tx.Rollback()

	var pr models.PullRequest
	err = tx.QueryRowContext(r.Context(), `
		SELECT id, project_id, global_values_name, author_id, title, description, status,
		       is_conflicted, created_at, updated_at, merged_at, closed_at
		FROM pull_requests
		WHERE project_id = $1 AND author_id = $2 AND status IN ('draft', 'open', 'approved')
		ORDER BY created_at DESC LIMIT 1
		FOR UPDATE
	`, projectID, user.UserID).Scan(
		&pr.ID, &pr.ProjectID, &pr.GlobalValuesName, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt, &pr.MergedAt, &pr.ClosedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "no active workspace", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	res, err := tx.ExecContext(r.Context(), `DELETE FROM pr_changes WHERE id = $1 AND pr_id = $2`, changeID, pr.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "staged change not found", "not_found")
		return
	}

	if pr.Status == "approved" {
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE pr_approvals SET withdrawn_at = NOW() WHERE pr_id = $1 AND withdrawn_at IS NULL
		`, pr.ID); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE pull_requests SET status = 'open', updated_at = NOW() WHERE id = $1
		`, pr.ID); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		pr.Status = "open"
	} else {
		if _, err = tx.ExecContext(r.Context(), `UPDATE pull_requests SET updated_at = NOW() WHERE id = $1`, pr.ID); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
	}

	if err = tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, msgFailedToCommit, "internal")
		return
	}

	pr.Changes, _ = h.loadChanges(r.Context(), pr.ID)
	writeJSON(w, http.StatusOK, pr)
}

// ── Overlay reads (base + the caller's own staged changes) ──────────────────

// stagedChangesByKey returns the active workspace's changes for one object
// type, keyed by template/environment name.
func (h *PullRequestHandler) stagedChangesByKey(ctx context.Context, projectID, userID int64, objectType string) (map[string]models.PRChange, error) {
	out := map[string]models.PRChange{}
	prID, ok, err := h.activeDraftID(ctx, projectID, userID)
	if err != nil || !ok {
		return out, err
	}
	changes, err := h.loadChanges(ctx, prID)
	if err != nil {
		return out, err
	}
	for _, c := range changes {
		if c.ObjectType != objectType {
			continue
		}
		switch objectType {
		case "template":
			if c.TemplateName != nil {
				out[*c.TemplateName] = c
			}
		default: // values, environment keyed by environment name
			if c.EnvironmentName != nil {
				out[*c.EnvironmentName] = c
			}
		}
	}
	return out, nil
}

// effectiveTemplates returns the workspace overlay templates: live latest of
// each template merged with the caller's staged changes, with staged deletes
// hidden. Staged-new templates have VersionID 0.
func (h *PullRequestHandler) effectiveTemplates(ctx context.Context, projectID, userID int64) ([]models.WorkspaceTemplateItem, error) {
	rows, err := h.DB.QueryContext(ctx, `
		SELECT DISTINCT ON (template_name) template_name, version_id, body
		FROM project_config_templates
		WHERE project_id = $1
		ORDER BY template_name, version_id DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type liveTmpl struct {
		version int
		body    string
	}
	live := map[string]liveTmpl{}
	var order []string
	for rows.Next() {
		var name, body string
		var version int
		if err := rows.Scan(&name, &version, &body); err != nil {
			return nil, err
		}
		live[name] = liveTmpl{version: version, body: body}
		order = append(order, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	staged, err := h.stagedChangesByKey(ctx, projectID, userID, "template")
	if err != nil {
		return nil, err
	}

	items := []models.WorkspaceTemplateItem{}
	for _, name := range order {
		c, ok := staged[name]
		if ok && c.Operation == "delete" {
			continue // hidden in the overlay
		}
		item := models.WorkspaceTemplateItem{TemplateName: name, Body: live[name].body, VersionID: live[name].version}
		if ok {
			item.Body = c.ProposedPayload
			item.Staged = true
			item.Operation = c.Operation
		}
		items = append(items, item)
	}
	// Staged-new templates (no live counterpart).
	for name, c := range staged {
		if _, isLive := live[name]; isLive {
			continue
		}
		if c.Operation == "delete" {
			continue
		}
		items = append(items, models.WorkspaceTemplateItem{
			TemplateName: name, Body: c.ProposedPayload, VersionID: 0, Staged: true, Operation: c.Operation,
		})
	}
	return items, nil
}

func (h *PullRequestHandler) OverlayTemplates(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projectName := chi.URLParam(r, "projectName")
	projectID, err := h.resolveProjectIDByName(r.Context(), projectName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	items, err := h.effectiveTemplates(r.Context(), projectID, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.WorkspaceTemplateItem]{Items: items, Count: len(items)})
}

func (h *PullRequestHandler) OverlayTemplate(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projectName := chi.URLParam(r, "projectName")
	templateName := chi.URLParam(r, "templateName")
	projectID, err := h.resolveProjectIDByName(r.Context(), projectName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	staged, err := h.stagedChangesByKey(r.Context(), projectID, user.UserID, "template")
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	var liveVersion int
	var liveBody string
	liveErr := h.DB.QueryRowContext(r.Context(), `
		SELECT version_id, body FROM project_config_templates
		WHERE project_id = $1 AND template_name = $2
		ORDER BY version_id DESC LIMIT 1
	`, projectID, templateName).Scan(&liveVersion, &liveBody)
	if liveErr != nil && liveErr != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	if c, ok := staged[templateName]; ok {
		if c.Operation == "delete" {
			writeError(w, http.StatusNotFound, "template deleted in workspace", "not_found")
			return
		}
		writeJSON(w, http.StatusOK, models.WorkspaceTemplateItem{
			TemplateName: templateName, Body: c.ProposedPayload, VersionID: liveVersion, Staged: true, Operation: c.Operation,
		})
		return
	}
	if liveErr == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "template not found", "not_found")
		return
	}
	writeJSON(w, http.StatusOK, models.WorkspaceTemplateItem{
		TemplateName: templateName, Body: liveBody, VersionID: liveVersion,
	})
}

// effectiveEnvironments returns the workspace overlay environments: live
// environments merged with the caller's staged changes, with staged deletes
// hidden.
func (h *PullRequestHandler) effectiveEnvironments(ctx context.Context, projectID, userID int64) ([]models.WorkspaceEnvironmentItem, error) {
	rows, err := h.DB.QueryContext(ctx,
		`SELECT name FROM environments WHERE project_id = $1 ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	live := map[string]bool{}
	var order []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		live[name] = true
		order = append(order, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	staged, err := h.stagedChangesByKey(ctx, projectID, userID, "environment")
	if err != nil {
		return nil, err
	}

	items := []models.WorkspaceEnvironmentItem{}
	for _, name := range order {
		c, ok := staged[name]
		if ok && c.Operation == "delete" {
			continue
		}
		item := models.WorkspaceEnvironmentItem{Name: name}
		if ok {
			item.Staged = true
			item.Operation = c.Operation
		}
		items = append(items, item)
	}
	for name, c := range staged {
		if live[name] || c.Operation == "delete" {
			continue
		}
		items = append(items, models.WorkspaceEnvironmentItem{Name: name, Staged: true, Operation: c.Operation})
	}
	return items, nil
}

func (h *PullRequestHandler) OverlayEnvironments(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projectName := chi.URLParam(r, "projectName")
	projectID, err := h.resolveProjectIDByName(r.Context(), projectName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	items, err := h.effectiveEnvironments(r.Context(), projectID, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.WorkspaceEnvironmentItem]{Items: items, Count: len(items)})
}

// valuesState describes the workspace state of an environment's values.
type valuesState int

const (
	valuesPresent valuesState = iota // effective values exist (live or staged upsert)
	valuesDeleted                    // staged for deletion in the workspace
	valuesAbsent                     // none live and none staged
)

// effectiveValues returns the workspace overlay values for one environment: the
// caller's staged change if any, otherwise the live latest. The returned state
// distinguishes present values from a staged delete and from absence entirely.
func (h *PullRequestHandler) effectiveValues(ctx context.Context, projectID, userID int64, envName string) (models.WorkspaceValuesResponse, valuesState, error) {
	staged, err := h.stagedChangesByKey(ctx, projectID, userID, "values")
	if err != nil {
		return models.WorkspaceValuesResponse{}, valuesAbsent, err
	}

	var liveVersion int
	var livePayload json.RawMessage
	liveErr := h.DB.QueryRowContext(ctx, `
		SELECT v.version_id, v.payload
		FROM project_config_values v
		JOIN environments e ON e.id = v.environment_id
		WHERE v.project_id = $1 AND e.name = $2
		ORDER BY v.version_id DESC LIMIT 1
	`, projectID, envName).Scan(&liveVersion, &livePayload)
	if liveErr != nil && liveErr != sql.ErrNoRows {
		return models.WorkspaceValuesResponse{}, valuesAbsent, liveErr
	}

	if c, ok := staged[envName]; ok {
		if c.Operation == "delete" {
			return models.WorkspaceValuesResponse{}, valuesDeleted, nil
		}
		return models.WorkspaceValuesResponse{
			EnvironmentName: envName, Payload: json.RawMessage(c.ProposedPayload), VersionID: liveVersion, Staged: true, Operation: c.Operation,
		}, valuesPresent, nil
	}
	if liveErr == sql.ErrNoRows {
		return models.WorkspaceValuesResponse{}, valuesAbsent, nil
	}
	return models.WorkspaceValuesResponse{
		EnvironmentName: envName, Payload: livePayload, VersionID: liveVersion,
	}, valuesPresent, nil
}

func (h *PullRequestHandler) OverlayValues(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projectName := chi.URLParam(r, "projectName")
	envName := chi.URLParam(r, "envName")
	projectID, err := h.resolveProjectIDByName(r.Context(), projectName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	resp, state, err := h.effectiveValues(r.Context(), projectID, user.UserID, envName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	switch state {
	case valuesDeleted:
		writeError(w, http.StatusNotFound, "values deleted in workspace", "not_found")
	case valuesAbsent:
		writeError(w, http.StatusNotFound, "values not found", "not_found")
	default:
		writeJSON(w, http.StatusOK, resp)
	}
}
