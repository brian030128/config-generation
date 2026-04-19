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
func resolveEnvironmentID(r *http.Request, db *sql.DB, projectID int64, envName string) (int64, error) {
	var id int64
	err := db.QueryRowContext(r.Context(), `SELECT id FROM environments WHERE project_id = $1 AND name = $2`, projectID, envName).Scan(&id)
	return id, err
}

// Create creates a new value set (v1) for a (project, env) pair.
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
	if req.EnvironmentID == 0 || len(req.Payload) == 0 {
		writeError(w, http.StatusBadRequest, "environment_id and payload are required", "validation")
		return
	}

	var vals models.ProjectConfigValues
	err = h.DB.QueryRowContext(r.Context(), `
		INSERT INTO project_config_values (project_id, environment_id, version_id, payload, commit_message, created_by)
		VALUES ($1, $2, 1, $3, $4, $5)
		RETURNING id, project_id, environment_id, version_id, payload, commit_message, created_by, created_at
	`, projectID, req.EnvironmentID, req.Payload, req.CommitMessage, user.UserID).Scan(
		&vals.ID, &vals.ProjectID, &vals.EnvironmentID,
		&vals.VersionID, &vals.Payload, &vals.CommitMessage, &vals.CreatedBy, &vals.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "value set already exists for this environment", "conflict")
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

	envName := chi.URLParam(r, "envName")

	envID, err := resolveEnvironmentID(r, h.DB, projectID, envName)
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
			 WHERE project_id = $1 AND environment_id = $2
			 ORDER BY version_id DESC LIMIT 1
			 FOR UPDATE),
		0) + 1
	`, projectID, envID).Scan(&nextVersion)
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
		INSERT INTO project_config_values (project_id, environment_id, version_id, payload, commit_message, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, project_id, environment_id, version_id, payload, commit_message, created_by, created_at
	`, projectID, envID, nextVersion, req.Payload, req.CommitMessage, user.UserID).Scan(
		&vals.ID, &vals.ProjectID, &vals.EnvironmentID,
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

// GetLatest returns the latest version of a value set for (project, env).
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

	envName := chi.URLParam(r, "envName")

	envID, err := resolveEnvironmentID(r, h.DB, projectID, envName)
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
		SELECT id, project_id, environment_id, version_id, payload, commit_message, created_by, created_at
		FROM project_config_values
		WHERE project_id = $1 AND environment_id = $2
		ORDER BY version_id DESC LIMIT 1
	`, projectID, envID).Scan(
		&vals.ID, &vals.ProjectID, &vals.EnvironmentID,
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

	envName := chi.URLParam(r, "envName")

	envID, err := resolveEnvironmentID(r, h.DB, projectID, envName)
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
		SELECT id, project_id, environment_id, version_id, payload, commit_message, created_by, created_at
		FROM project_config_values
		WHERE project_id = $1 AND environment_id = $2 AND version_id = $3
	`, projectID, envID, versionID).Scan(
		&vals.ID, &vals.ProjectID, &vals.EnvironmentID,
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
