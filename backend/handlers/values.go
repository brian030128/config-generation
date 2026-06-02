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

// resolveProjectAndEnv resolves both the project ID (from the route) and the
// environment ID (from chi URLParam "envName"). On error it writes the
// appropriate HTTP response and returns ok=false; callers should just return.
func (h *ValuesHandler) resolveProjectAndEnv(w http.ResponseWriter, r *http.Request) (projectID, envID int64, ok bool) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return 0, 0, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return 0, 0, false
	}

	envID, err = resolveEnvironmentID(r, h.DB, projectID, chi.URLParam(r, "envName"))
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "environment not found", "not_found")
		return 0, 0, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return 0, 0, false
	}
	return projectID, envID, true
}

// scanValues decodes a project_config_values row into vals.
func scanValues(row *sql.Row, vals *models.ProjectConfigValues) error {
	return row.Scan(
		&vals.ID, &vals.ProjectID, &vals.EnvironmentID, &vals.VersionID,
		&vals.Payload, &vals.CommitMessage, &vals.CreatedBy, &vals.CreatedAt,
	)
}

// writeValuesOr404 writes vals as JSON, or a 404/500 if err is non-nil.
func writeValuesOr404(w http.ResponseWriter, err error, notFoundMsg string, vals models.ProjectConfigValues) {
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, notFoundMsg, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, vals)
}

// GetLatest returns the values for (project, env) as of the project's latest version.
func (h *ValuesHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	projectID, envID, ok := h.resolveProjectAndEnv(w, r)
	if !ok {
		return
	}

	var vals models.ProjectConfigValues
	err := scanValues(h.DB.QueryRowContext(r.Context(), `
		SELECT v.id, v.project_id, v.environment_id, v.version_id, v.payload, v.commit_message, v.created_by, v.created_at
		FROM project_version_values pvv
		JOIN project_config_values v ON v.id = pvv.values_row_id
		WHERE pvv.environment_id = $2
		  AND pvv.project_version_id = (
		      SELECT id FROM project_versions
		      WHERE project_id = $1 AND NOT is_anchor
		      ORDER BY version_id DESC LIMIT 1
		  )
	`, projectID, envID), &vals)
	writeValuesOr404(w, err, "values not found", vals)
}

// GetVersion returns the values pinned to a specific project version.
func (h *ValuesHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	projectID, envID, ok := h.resolveProjectAndEnv(w, r)
	if !ok {
		return
	}

	versionID, err := strconv.Atoi(chi.URLParam(r, "versionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version ID", "bad_request")
		return
	}

	var vals models.ProjectConfigValues
	err = scanValues(h.DB.QueryRowContext(r.Context(), `
		SELECT v.id, v.project_id, v.environment_id, v.version_id, v.payload, v.commit_message, v.created_by, v.created_at
		FROM project_versions pv
		JOIN project_version_values pvv ON pvv.project_version_id = pv.id
		JOIN project_config_values v ON v.id = pvv.values_row_id
		WHERE pv.project_id = $1 AND pv.version_id = $2 AND pvv.environment_id = $3
	`, projectID, versionID, envID), &vals)
	writeValuesOr404(w, err, "values version not found", vals)
}
