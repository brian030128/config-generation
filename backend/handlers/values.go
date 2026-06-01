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

// GetLatest returns the latest version of a value set for (project, env).
func (h *ValuesHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	envName := chi.URLParam(r, "envName")

	envID, err := resolveEnvironmentID(r, h.DB, projectID, envName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "environment not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
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
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, vals)
}

// GetVersion returns a specific version of a value set.
func (h *ValuesHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	envName := chi.URLParam(r, "envName")

	envID, err := resolveEnvironmentID(r, h.DB, projectID, envName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "environment not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
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
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, vals)
}
