package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/brian/config-generation/backend/models"
	"github.com/go-chi/chi/v5"
)

type TemplateHandler struct {
	DB *sql.DB
}

// resolveProjectID looks up the project ID by name. Returns 0 if not found.
func resolveProjectID(r *http.Request, db *sql.DB) (int64, error) {
	projectName := chi.URLParam(r, "projectName")
	var id int64
	err := db.QueryRowContext(r.Context(), `SELECT id FROM projects WHERE name = $1`, projectName).Scan(&id)
	return id, err
}

// Create creates a new template with version 1.
func (h *TemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req models.CreateTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	if req.TemplateName == "" || req.Body == "" {
		writeError(w, http.StatusBadRequest, "template_name and body are required", "validation")
		return
	}

	var tmpl models.ProjectConfigTemplate
	err = h.DB.QueryRowContext(r.Context(), `
		INSERT INTO project_config_templates (project_id, template_name, version_id, body, commit_message, created_by)
		VALUES ($1, $2, 1, $3, $4, $5)
		RETURNING id, project_id, template_name, version_id, body, commit_message, created_by, created_at
	`, projectID, req.TemplateName, req.Body, req.CommitMessage, user.UserID).Scan(
		&tmpl.ID, &tmpl.ProjectID, &tmpl.TemplateName, &tmpl.VersionID,
		&tmpl.Body, &tmpl.CommitMessage, &tmpl.CreatedBy, &tmpl.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "template already exists", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create template", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, tmpl)
}

// AppendVersion appends a new version to an existing template.
func (h *TemplateHandler) AppendVersion(w http.ResponseWriter, r *http.Request) {
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

	var req models.AppendTemplateVersionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	if req.Body == "" {
		writeError(w, http.StatusBadRequest, "body is required", "validation")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer tx.Rollback()

	// Get next version ID with row lock.
	var nextVersion int
	err = tx.QueryRowContext(r.Context(), `
		SELECT COALESCE(
			(SELECT version_id FROM project_config_templates
			 WHERE project_id = $1 AND template_name = $2
			 ORDER BY version_id DESC LIMIT 1
			 FOR UPDATE),
		0) + 1
	`, projectID, templateName).Scan(&nextVersion)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	if nextVersion == 1 {
		writeError(w, http.StatusNotFound, "template not found, use create instead", "not_found")
		return
	}

	var tmpl models.ProjectConfigTemplate
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO project_config_templates (project_id, template_name, version_id, body, commit_message, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, project_id, template_name, version_id, body, commit_message, created_by, created_at
	`, projectID, templateName, nextVersion, req.Body, req.CommitMessage, user.UserID).Scan(
		&tmpl.ID, &tmpl.ProjectID, &tmpl.TemplateName, &tmpl.VersionID,
		&tmpl.Body, &tmpl.CommitMessage, &tmpl.CreatedBy, &tmpl.CreatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to append version", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, tmpl)
}

// GetLatest returns the latest version of a template.
func (h *TemplateHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
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

	var tmpl models.ProjectConfigTemplate
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, project_id, template_name, version_id, body, commit_message, created_by, created_at
		FROM project_config_templates
		WHERE project_id = $1 AND template_name = $2
		ORDER BY version_id DESC LIMIT 1
	`, projectID, templateName).Scan(
		&tmpl.ID, &tmpl.ProjectID, &tmpl.TemplateName, &tmpl.VersionID,
		&tmpl.Body, &tmpl.CommitMessage, &tmpl.CreatedBy, &tmpl.CreatedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "template not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	writeJSON(w, http.StatusOK, tmpl)
}

// GetVersion returns a specific version of a template.
func (h *TemplateHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
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
	versionID, err := strconv.Atoi(chi.URLParam(r, "versionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version ID", "bad_request")
		return
	}

	var tmpl models.ProjectConfigTemplate
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, project_id, template_name, version_id, body, commit_message, created_by, created_at
		FROM project_config_templates
		WHERE project_id = $1 AND template_name = $2 AND version_id = $3
	`, projectID, templateName, versionID).Scan(
		&tmpl.ID, &tmpl.ProjectID, &tmpl.TemplateName, &tmpl.VersionID,
		&tmpl.Body, &tmpl.CommitMessage, &tmpl.CreatedBy, &tmpl.CreatedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "template version not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	writeJSON(w, http.StatusOK, tmpl)
}

// ListForProject returns the latest version of each template in a project.
func (h *TemplateHandler) ListForProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT DISTINCT ON (template_name)
			id, project_id, template_name, version_id, body, commit_message, created_by, created_at
		FROM project_config_templates
		WHERE project_id = $1
		ORDER BY template_name, version_id DESC
	`, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer rows.Close()

	var templates []models.ProjectConfigTemplate
	for rows.Next() {
		var t models.ProjectConfigTemplate
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.TemplateName, &t.VersionID, &t.Body, &t.CommitMessage, &t.CreatedBy, &t.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	if templates == nil {
		templates = []models.ProjectConfigTemplate{}
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.ProjectConfigTemplate]{Items: templates, Count: len(templates)})
}

// ListVersions returns all versions of a template.
func (h *TemplateHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
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

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, project_id, template_name, version_id, body, commit_message, created_by, created_at
		FROM project_config_templates
		WHERE project_id = $1 AND template_name = $2
		ORDER BY version_id DESC
	`, projectID, templateName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer rows.Close()

	var versions []models.ProjectConfigTemplate
	for rows.Next() {
		var t models.ProjectConfigTemplate
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.TemplateName, &t.VersionID, &t.Body, &t.CommitMessage, &t.CreatedBy, &t.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		versions = append(versions, t)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	if versions == nil {
		versions = []models.ProjectConfigTemplate{}
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.ProjectConfigTemplate]{Items: versions, Count: len(versions)})
}
