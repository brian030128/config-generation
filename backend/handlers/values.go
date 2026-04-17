package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/brian/config-generation/backend/models"
	"github.com/go-chi/chi/v5"
)

type ValuesHandler struct {
	DB *sql.DB
}

// resolveEnvironmentID looks up the environment ID by name.
func resolveEnvironmentID(r *http.Request, db *sql.DB, envName string) (int64, error) {
	var id int64
	err := db.QueryRowContext(r.Context(), `SELECT id FROM environments WHERE name = $1`, envName).Scan(&id)
	return id, err
}

// Create creates a new value set (v1) for a (template, env) pair.
func (h *ValuesHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	var req models.CreateProjectConfigValuesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	if req.TemplateName == "" || req.EnvironmentID == 0 || len(req.Payload) == 0 {
		writeError(w, http.StatusBadRequest, "template_name, environment_id, and payload are required", "validation")
		return
	}

	var vals models.ProjectConfigValues
	err = h.DB.QueryRowContext(r.Context(), `
		INSERT INTO project_config_values (project_id, template_name, environment_id, version_id, payload, commit_message, created_by)
		VALUES ($1, $2, $3, 1, $4, $5, $6)
		RETURNING id, project_id, template_name, environment_id, version_id, payload, commit_message, created_by, created_at
	`, projectID, req.TemplateName, req.EnvironmentID, req.Payload, req.CommitMessage, user.UserID).Scan(
		&vals.ID, &vals.ProjectID, &vals.TemplateName, &vals.EnvironmentID,
		&vals.VersionID, &vals.Payload, &vals.CommitMessage, &vals.CreatedBy, &vals.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "value set already exists for this template and environment", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create values", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, vals)
}

// AppendVersion appends a new version to an existing value set.
func (h *ValuesHandler) AppendVersion(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	templateName := chi.URLParam(r, "templateName")
	envName := chi.URLParam(r, "envName")

	envID, err := resolveEnvironmentID(r, h.DB, envName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "environment not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	var req models.AppendProjectConfigValuesVersionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	if len(req.Payload) == 0 {
		writeError(w, http.StatusBadRequest, "payload is required", "validation")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer tx.Rollback()

	var nextVersion int
	err = tx.QueryRowContext(r.Context(), `
		SELECT COALESCE(
			(SELECT version_id FROM project_config_values
			 WHERE project_id = $1 AND template_name = $2 AND environment_id = $3
			 ORDER BY version_id DESC LIMIT 1
			 FOR UPDATE),
		0) + 1
	`, projectID, templateName, envID).Scan(&nextVersion)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	if nextVersion == 1 {
		writeError(w, http.StatusNotFound, "value set not found, use create instead", "not_found")
		return
	}

	var vals models.ProjectConfigValues
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO project_config_values (project_id, template_name, environment_id, version_id, payload, commit_message, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, project_id, template_name, environment_id, version_id, payload, commit_message, created_by, created_at
	`, projectID, templateName, envID, nextVersion, req.Payload, req.CommitMessage, user.UserID).Scan(
		&vals.ID, &vals.ProjectID, &vals.TemplateName, &vals.EnvironmentID,
		&vals.VersionID, &vals.Payload, &vals.CommitMessage, &vals.CreatedBy, &vals.CreatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to append version", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, vals)
}

// GetLatest returns the latest version of a value set for (template, env).
func (h *ValuesHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	templateName := chi.URLParam(r, "templateName")
	envName := chi.URLParam(r, "envName")

	envID, err := resolveEnvironmentID(r, h.DB, envName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "environment not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	var vals models.ProjectConfigValues
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, project_id, template_name, environment_id, version_id, payload, commit_message, created_by, created_at
		FROM project_config_values
		WHERE project_id = $1 AND template_name = $2 AND environment_id = $3
		ORDER BY version_id DESC LIMIT 1
	`, projectID, templateName, envID).Scan(
		&vals.ID, &vals.ProjectID, &vals.TemplateName, &vals.EnvironmentID,
		&vals.VersionID, &vals.Payload, &vals.CommitMessage, &vals.CreatedBy, &vals.CreatedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "values not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	writeJSON(w, http.StatusOK, vals)
}

// GetVersion returns a specific version of a value set.
func (h *ValuesHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	templateName := chi.URLParam(r, "templateName")
	envName := chi.URLParam(r, "envName")

	envID, err := resolveEnvironmentID(r, h.DB, envName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "environment not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	versionID, err := strconv.Atoi(chi.URLParam(r, "versionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version ID", "bad_request")
		return
	}

	var vals models.ProjectConfigValues
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, project_id, template_name, environment_id, version_id, payload, commit_message, created_by, created_at
		FROM project_config_values
		WHERE project_id = $1 AND template_name = $2 AND environment_id = $3 AND version_id = $4
	`, projectID, templateName, envID, versionID).Scan(
		&vals.ID, &vals.ProjectID, &vals.TemplateName, &vals.EnvironmentID,
		&vals.VersionID, &vals.Payload, &vals.CommitMessage, &vals.CreatedBy, &vals.CreatedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "values version not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	writeJSON(w, http.StatusOK, vals)
}

// ListForProjectEnv returns the latest version of each value set for a (project, env) pair.
func (h *ValuesHandler) ListForProjectEnv(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	envName := chi.URLParam(r, "envName")
	envID, err := resolveEnvironmentID(r, h.DB, envName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "environment not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT DISTINCT ON (template_name)
			id, project_id, template_name, environment_id, version_id, payload, commit_message, created_by, created_at
		FROM project_config_values
		WHERE project_id = $1 AND environment_id = $2
		ORDER BY template_name, version_id DESC
	`, projectID, envID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer rows.Close()

	var values []models.ProjectConfigValues
	for rows.Next() {
		var v models.ProjectConfigValues
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.TemplateName, &v.EnvironmentID, &v.VersionID, &v.Payload, &v.CommitMessage, &v.CreatedBy, &v.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		values = append(values, v)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	if values == nil {
		values = []models.ProjectConfigValues{}
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.ProjectConfigValues]{Items: values, Count: len(values)})
}
