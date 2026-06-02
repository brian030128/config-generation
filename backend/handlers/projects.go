package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/brian/config-generation/backend/middleware"
	"github.com/brian/config-generation/backend/models"
	"github.com/go-chi/chi/v5"
)

type ProjectHandler struct {
	DB *sql.DB
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	var req models.CreateProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required", "validation")
		return
	}

	// Roles are global; a project's auto-created admin role is named
	// "<project>_project_admin". The approval condition defaults to requiring it
	// and, at creation, may only reference it (other approver roles are created on
	// the global Roles page afterwards, then the condition is edited).
	adminRoleName := req.Name + "_project_admin"
	cond := "1 x " + adminRoleName
	if req.ApprovalCondition != nil {
		cond = *req.ApprovalCondition
	}
	if err := validateApprovalCondition(r.Context(), h.DB, []string{adminRoleName}, cond); err != nil {
		writeApprovalConditionError(w, err)
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tx.Rollback()

	// 1. Insert project.
	var project models.Project
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO projects (name, description, approval_condition, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, description, approval_condition, created_by, created_at, updated_at
	`, req.Name, req.Description, cond, user.UserID).Scan(
		&project.ID, &project.Name, &project.Description,
		&project.ApprovalCondition, &project.CreatedBy,
		&project.CreatedAt, &project.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "project name already exists", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create project", "internal")
		return
	}

	// 2. Auto-create the project's admin role (global; scope comes from its atoms).
	var roleID int64
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO roles (name, is_auto_created) VALUES ($1, true)
		RETURNING id
	`, adminRoleName).Scan(&roleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create admin role", "internal")
		return
	}

	// 3. Insert permission atoms for the admin role.
	adminPerms := []struct {
		action, resource, keyProject, keyEnv string
	}{
		{"write", "project_templates", project.Name, ""},
		{"delete", "project_templates", project.Name, ""},
		// Wildcard env so the admin can create value sets (and new environments)
		// in every env; env-admins get an env-scoped create:env_values(p, env).
		{"create", "env_values", project.Name, "*"},
		{"delete", "project_values", project.Name, "*"},
		{"delete", "project", project.Name, ""},
		{"grant", "", project.Name, ""},
	}
	for _, p := range adminPerms {
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO role_permissions (role_id, action, resource, key_project, key_env)
			VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''))
		`, roleID, p.action, p.resource, p.keyProject, p.keyEnv)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create role permissions", "internal")
			return
		}
	}

	// 4. Assign role to creator.
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO user_roles (user_id, role_id, granted_by) VALUES ($1, $2, $3)
	`, user.UserID, roleID, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to assign admin role", "internal")
		return
	}

	// 5. Add the creator as a project member (grants read:project). The creator
	// is both admin and member.
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO project_members (project_id, user_id, added_by) VALUES ($1, $2, $2)
	`, project.ID, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add creator as member", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, msgFailedToCommit, "internal")
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

// UpdateApprovalCondition changes a project's PR-approval condition. The new
// condition must be parseable and may only reference roles that exist in the
// project (plus the built-in project_admin).
func (h *ProjectHandler) UpdateApprovalCondition(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "projectName")

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

	var req models.UpdateApprovalConditionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}

	if err := validateApprovalCondition(r.Context(), h.DB, []string{projectName + "_project_admin"}, req.ApprovalCondition); err != nil {
		writeApprovalConditionError(w, err)
		return
	}

	var project models.Project
	err = h.DB.QueryRowContext(r.Context(), `
		UPDATE projects SET approval_condition = $1, updated_at = now() WHERE id = $2
		RETURNING id, name, description, approval_condition, created_by, created_at, updated_at
	`, strings.TrimSpace(req.ApprovalCondition), projectID).Scan(
		&project.ID, &project.Name, &project.Description,
		&project.ApprovalCondition, &project.CreatedBy,
		&project.CreatedAt, &project.UpdatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update approval condition", "internal")
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "projectName")

	var project models.Project
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT id, name, description, approval_condition, created_by, created_at, updated_at
		FROM projects WHERE name = $1
	`, projectName).Scan(
		&project.ID, &project.Name, &project.Description,
		&project.ApprovalCondition, &project.CreatedBy,
		&project.CreatedAt, &project.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	// Scope the list to projects the caller can read: superusers see all,
	// everyone else sees only projects they hold read:project on (i.e. are a
	// member of, or hold a wildcard read:project).
	su, err := middleware.IsSuperuser(r.Context(), h.DB, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check user", "internal")
		return
	}

	var perms middleware.EffectivePermissionSet
	if !su {
		perms, err = middleware.LoadEffectivePermissions(r.Context(), h.DB, user.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load permissions", "internal")
			return
		}
	}

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, name, description, approval_condition, created_by, created_at, updated_at
		FROM projects ORDER BY updated_at DESC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer rows.Close()

	projects := []models.Project{}
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.ApprovalCondition, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		if !su && !perms.Can(models.PermissionRequirement{
			Action:     models.ActionRead,
			Resource:   models.ResourceProject,
			KeyProject: p.Name,
		}) {
			continue
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.Project]{Items: projects, Count: len(projects)})
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tx.Rollback()

	// Delete dependent rows in order to satisfy FK constraints.
	cascadeQueries := []string{
		// deployments: clear self-reference + group refs first, then child tables
		`UPDATE deployments SET rolled_back_from = NULL WHERE project_id = $1`,
		`DELETE FROM deployment_group_refs WHERE deployment_id IN (SELECT id FROM deployments WHERE project_id = $1)`,
		`DELETE FROM deployment_entries WHERE deployment_id IN (SELECT id FROM deployments WHERE project_id = $1)`,
		`DELETE FROM deployments WHERE project_id = $1`,
		// pull requests and their child tables
		`DELETE FROM pr_approvals WHERE pr_id IN (SELECT id FROM pull_requests WHERE project_id = $1)`,
		`DELETE FROM pr_changes WHERE pr_id IN (SELECT id FROM pull_requests WHERE project_id = $1)`,
		`DELETE FROM pull_requests WHERE project_id = $1`,
		// members
		`DELETE FROM project_members WHERE project_id = $1`,
		// project version snapshot rows must go before the content rows they
		// reference, which in turn must go before environments.
		`DELETE FROM project_version_values WHERE project_version_id IN (SELECT id FROM project_versions WHERE project_id = $1)`,
		`DELETE FROM project_version_templates WHERE project_version_id IN (SELECT id FROM project_versions WHERE project_id = $1)`,
		`DELETE FROM project_versions WHERE project_id = $1`,
		`DELETE FROM project_config_values WHERE project_id = $1`,
		`DELETE FROM project_config_templates WHERE project_id = $1`,
		`DELETE FROM env_admins WHERE environment_id IN (SELECT id FROM environments WHERE project_id = $1)`,
		`DELETE FROM environments WHERE project_id = $1`,
		`DELETE FROM projects WHERE id = $1`,
	}
	for _, q := range cascadeQueries {
		if _, err := tx.ExecContext(r.Context(), q, projectID); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
	}

	// Roles are global; remove only this project's auto admin role
	// ("<name>_project_admin") and its per-member managed roles
	// ("__member__:<name>:*"), identified by name. Other roles are left intact.
	const projectRolesFilter = `name = $1 || '_project_admin' OR name LIKE '__member__:' || $1 || ':%'`
	for _, q := range []string{
		`DELETE FROM user_roles WHERE role_id IN (SELECT id FROM roles WHERE ` + projectRolesFilter + `)`,
		`DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM roles WHERE ` + projectRolesFilter + `)`,
		`DELETE FROM roles WHERE ` + projectRolesFilter,
	} {
		if _, err := tx.ExecContext(r.Context(), q, projectName); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, msgFailedToCommit, "internal")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListVersions returns the project's version history, newest first.
func (h *ProjectHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, project_id, version_id, parent_version_id, commit_message, is_anchor, created_by, created_at
		FROM project_versions
		WHERE project_id = $1
		ORDER BY version_id DESC
	`, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer rows.Close()

	versions := []models.ProjectVersion{}
	for rows.Next() {
		var v models.ProjectVersion
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.VersionID, &v.ParentVersionID, &v.CommitMessage, &v.IsAnchor, &v.CreatedBy, &v.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.ProjectVersion]{Items: versions, Count: len(versions)})
}

// GetVersion returns the manifest of one project version: its metadata plus
// the materialized list of templates and values it snapshots.
func (h *ProjectHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	versionID := chi.URLParam(r, "versionID")

	var manifest models.ProjectVersionManifest
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, project_id, version_id, parent_version_id, commit_message, is_anchor, created_by, created_at
		FROM project_versions
		WHERE project_id = $1 AND version_id = $2::int
	`, projectID, versionID).Scan(
		&manifest.ID, &manifest.ProjectID, &manifest.VersionID, &manifest.ParentVersionID,
		&manifest.CommitMessage, &manifest.IsAnchor, &manifest.CreatedBy, &manifest.CreatedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project version not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	tmplRows, err := h.DB.QueryContext(r.Context(), `
		SELECT t.id, t.project_id, t.template_name, t.version_id, t.body, t.commit_message, t.created_by, t.created_at
		FROM project_version_templates pvt
		JOIN project_config_templates t ON t.id = pvt.template_row_id
		WHERE pvt.project_version_id = $1
		ORDER BY t.template_name
	`, manifest.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tmplRows.Close()
	manifest.Templates = []models.ProjectConfigTemplate{}
	for tmplRows.Next() {
		var t models.ProjectConfigTemplate
		if err := tmplRows.Scan(&t.ID, &t.ProjectID, &t.TemplateName, &t.VersionID, &t.Body, &t.CommitMessage, &t.CreatedBy, &t.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		manifest.Templates = append(manifest.Templates, t)
	}

	valRows, err := h.DB.QueryContext(r.Context(), `
		SELECT v.id, v.project_id, v.environment_id, v.version_id, v.payload, v.commit_message, v.created_by, v.created_at
		FROM project_version_values pvv
		JOIN project_config_values v ON v.id = pvv.values_row_id
		WHERE pvv.project_version_id = $1
		ORDER BY v.environment_id
	`, manifest.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer valRows.Close()
	manifest.Values = []models.ProjectConfigValues{}
	for valRows.Next() {
		var v models.ProjectConfigValues
		if err := valRows.Scan(&v.ID, &v.ProjectID, &v.EnvironmentID, &v.VersionID, &v.Payload, &v.CommitMessage, &v.CreatedBy, &v.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		manifest.Values = append(manifest.Values, v)
	}

	writeJSON(w, http.StatusOK, manifest)
}
