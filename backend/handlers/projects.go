package handlers

import (
	"database/sql"
	"net/http"

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
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required", "validation")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer tx.Rollback()

	// 1. Insert project.
	var project models.Project
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO projects (name, description, approval_condition, created_by)
		VALUES ($1, $2, COALESCE($3, '1 x project_admin'), $4)
		RETURNING id, name, description, approval_condition, created_by, created_at, updated_at
	`, req.Name, req.Description, req.ApprovalCondition, user.UserID).Scan(
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

	// 2. Auto-create project_admin role.
	roleName := "project_admin:" + project.Name
	var roleID int64
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO roles (name, project_id, is_auto_created) VALUES ($1, $2, true)
		RETURNING id
	`, roleName, project.ID).Scan(&roleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create admin role", "internal")
		return
	}

	// 3. Insert permission atoms for the admin role.
	adminPerms := []struct {
		action, resource, keyProject, keyEnv string
	}{
		{"write", "project_templates", project.Name, ""},
		{"create", "env_values", project.Name, ""},
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

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, project)
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
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, name, description, approval_condition, created_by, created_at, updated_at
		FROM projects ORDER BY updated_at DESC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.ApprovalCondition, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	if projects == nil {
		projects = []models.Project{}
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.Project]{Items: projects, Count: len(projects)})
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "projectName")

	// Resolve project ID.
	var projectID int64
	err := h.DB.QueryRowContext(r.Context(), `SELECT id FROM projects WHERE name = $1`, projectName).Scan(&projectID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer tx.Rollback()

	// Delete dependent rows in order to satisfy FK constraints.
	cascadeQueries := []string{
		`DELETE FROM user_roles WHERE role_id IN (SELECT id FROM roles WHERE project_id = $1)`,
		`DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM roles WHERE project_id = $1)`,
		`DELETE FROM roles WHERE project_id = $1`,
		`DELETE FROM project_config_values WHERE project_id = $1`,
		`DELETE FROM project_config_templates WHERE project_id = $1`,
		`DELETE FROM environments WHERE project_id = $1`,
		`DELETE FROM projects WHERE id = $1`,
	}
	for _, q := range cascadeQueries {
		if _, err := tx.ExecContext(r.Context(), q, projectID); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
