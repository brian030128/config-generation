package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/brian/config-generation/backend/models"
	"github.com/brian/config-generation/backend/services"
	"github.com/go-chi/chi/v5"
)

type DeploymentHandler struct {
	DB *sql.DB
}

// Preview renders all templates with pinned versions and returns results plus
// previous deployment data for diff computation on the client.
func (h *DeploymentHandler) Preview(w http.ResponseWriter, r *http.Request) {
	projectID, envID, ok := h.resolveProjectEnv(w, r)
	if !ok {
		return
	}

	var req models.DeployPreviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	// Fetch templates at pinned versions
	templates, templateBodies, ok := h.fetchPinnedTemplates(w, r, projectID, req.TemplateVersions)
	if !ok {
		return
	}

	// Fetch values at pinned version
	valuesPayload, ok := h.fetchPinnedValues(w, r, projectID, envID, req.ValuesVersionID)
	if !ok {
		return
	}

	// Fetch global values at pinned versions
	gvMap, gvPayloads, ok := h.fetchPinnedGlobalValues(w, r, req.GlobalValuesVersions)
	if !ok {
		return
	}

	// Render all templates
	renderResults := services.RenderAll(templates, valuesPayload, gvMap)

	// Fetch last deployment for diff baseline
	prevTemplates, prevValues, prevGlobalValues := h.fetchLastDeploymentBaseline(r, projectID, envID)

	// Build response
	hasErrors := false
	results := make([]models.TemplateRenderResult, len(renderResults))
	for i, rr := range renderResults {
		results[i] = models.TemplateRenderResult{
			TemplateName:      rr.TemplateName,
			TemplateBody:      templateBodies[rr.TemplateName],
			TemplateVersionID: req.TemplateVersions[rr.TemplateName],
		}
		if rr.Error != nil {
			hasErrors = true
			errMsg := rr.Error.Message
			errKind := string(rr.Error.Kind)
			results[i].Error = &errMsg
			results[i].ErrorKind = &errKind
		} else {
			results[i].RenderedOutput = rr.RenderedOutput
		}
		if prev, ok := prevTemplates[rr.TemplateName]; ok {
			results[i].PreviousOutput = prev.RenderedOutput
			results[i].PreviousTemplateBody = prev.TemplateBody
		}
	}

	resp := models.DeployPreviewResponse{
		Results:              results,
		ValuesPayload:        valuesPayload,
		PreviousValues:       prevValues,
		ValuesVersionID:      req.ValuesVersionID,
		GlobalValues:         gvPayloads,
		PreviousGlobalValues: prevGlobalValues,
		GlobalValuesVersions: req.GlobalValuesVersions,
		HasErrors:            hasErrors,
	}

	writeJSON(w, http.StatusOK, resp)
}

// Execute renders all templates and, if successful, creates a deployment record
// and returns the rendered outputs for zip generation on the client.
func (h *DeploymentHandler) Execute(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projectID, envID, ok := h.resolveProjectEnv(w, r)
	if !ok {
		return
	}

	var req models.DeployRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	// Fetch templates at pinned versions
	templates, _, ok := h.fetchPinnedTemplates(w, r, projectID, req.TemplateVersions)
	if !ok {
		return
	}

	// Fetch values at pinned version
	valuesPayload, ok := h.fetchPinnedValues(w, r, projectID, envID, req.ValuesVersionID)
	if !ok {
		return
	}

	// Fetch global values at pinned versions
	gvMap, _, ok := h.fetchPinnedGlobalValues(w, r, req.GlobalValuesVersions)
	if !ok {
		return
	}

	// Render all templates
	renderResults := services.RenderAll(templates, valuesPayload, gvMap)

	// Check for errors
	for _, rr := range renderResults {
		if rr.Error != nil {
			writeError(w, http.StatusUnprocessableEntity, rr.Error.Message, string(rr.Error.Kind))
			return
		}
	}

	// Create deployment record in a transaction
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer tx.Rollback()

	var deploymentID int64
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO deployments (project_id, environment_id, status, commit_message, created_by)
		VALUES ($1, $2, 'succeeded', $3, $4)
		RETURNING id
	`, projectID, envID, req.CommitMessage, user.UserID).Scan(&deploymentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create deployment", "internal")
		return
	}

	// Insert deployment entries
	for _, rr := range renderResults {
		var entryID int64
		err = tx.QueryRowContext(r.Context(), `
			INSERT INTO deployment_entries (deployment_id, template_name, template_version_id, values_version_id, rendered_output)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, deploymentID, rr.TemplateName, req.TemplateVersions[rr.TemplateName], req.ValuesVersionID, *rr.RenderedOutput).Scan(&entryID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create deployment entry", "internal")
			return
		}

		// Insert global value refs for this entry
		for gvName, gvVersion := range req.GlobalValuesVersions {
			_, err = tx.ExecContext(r.Context(), `
				INSERT INTO deployment_entry_global_refs (deployment_entry_id, global_values_name, global_values_version_id)
				VALUES ($1, $2, $3)
			`, entryID, gvName, gvVersion)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to create global value ref", "internal")
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit deployment", "internal")
		return
	}

	// Build response
	results := make([]models.TemplateRenderResult, len(renderResults))
	for i, rr := range renderResults {
		results[i] = models.TemplateRenderResult{
			TemplateName:      rr.TemplateName,
			RenderedOutput:    rr.RenderedOutput,
			TemplateVersionID: req.TemplateVersions[rr.TemplateName],
		}
	}

	writeJSON(w, http.StatusCreated, models.DeployResponse{
		DeploymentID: deploymentID,
		Status:       "succeeded",
		Results:      results,
	})
}

// GetLatest returns the version set from the last successful deployment
// for a (project, environment) pair.
func (h *DeploymentHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	projectID, envID, ok := h.resolveProjectEnv(w, r)
	if !ok {
		return
	}

	var dep models.Deployment
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT id, commit_message, created_at
		FROM deployments
		WHERE project_id = $1 AND environment_id = $2 AND status = 'succeeded'
		ORDER BY created_at DESC LIMIT 1
	`, projectID, envID).Scan(&dep.ID, &dep.CommitMessage, &dep.CreatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "no successful deployment found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	// Fetch template and values versions from deployment entries
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT template_name, template_version_id, values_version_id
		FROM deployment_entries
		WHERE deployment_id = $1
	`, dep.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer rows.Close()

	templateVersions := map[string]int{}
	var valuesVersionID int
	for rows.Next() {
		var name string
		var tmplVer, valsVer int
		if err := rows.Scan(&name, &tmplVer, &valsVer); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		templateVersions[name] = tmplVer
		valuesVersionID = valsVer
	}

	// Fetch global values versions from any entry's refs
	gvRows, err := h.DB.QueryContext(r.Context(), `
		SELECT DISTINCT degr.global_values_name, degr.global_values_version_id
		FROM deployment_entry_global_refs degr
		JOIN deployment_entries de ON de.id = degr.deployment_entry_id
		WHERE de.deployment_id = $1
	`, dep.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer gvRows.Close()

	gvVersions := map[string]int{}
	for gvRows.Next() {
		var name string
		var ver int
		if err := gvRows.Scan(&name, &ver); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		gvVersions[name] = ver
	}

	writeJSON(w, http.StatusOK, models.LatestDeploymentResponse{
		DeploymentID:         dep.ID,
		TemplateVersions:     templateVersions,
		ValuesVersionID:      valuesVersionID,
		GlobalValuesVersions: gvVersions,
		CreatedAt:            dep.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		CommitMessage:        dep.CommitMessage,
	})
}

// --- helpers ---

func (h *DeploymentHandler) resolveProjectEnv(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return 0, 0, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return 0, 0, false
	}

	envName := chi.URLParam(r, "envName")
	envID, err := resolveEnvironmentID(r, h.DB, projectID, envName)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "environment not found", "not_found")
		return 0, 0, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return 0, 0, false
	}

	return projectID, envID, true
}

func (h *DeploymentHandler) fetchPinnedTemplates(w http.ResponseWriter, r *http.Request, projectID int64, versions map[string]int) ([]services.TemplateInput, map[string]string, bool) {
	templates := make([]services.TemplateInput, 0, len(versions))
	bodies := make(map[string]string, len(versions))

	for name, ver := range versions {
		var body string
		err := h.DB.QueryRowContext(r.Context(), `
			SELECT body FROM project_config_templates
			WHERE project_id = $1 AND template_name = $2 AND version_id = $3
		`, projectID, name, ver).Scan(&body)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "template "+name+" version not found", "not_found")
			return nil, nil, false
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return nil, nil, false
		}
		templates = append(templates, services.TemplateInput{Name: name, Body: body})
		bodies[name] = body
	}

	return templates, bodies, true
}

func (h *DeploymentHandler) fetchPinnedValues(w http.ResponseWriter, r *http.Request, projectID, envID int64, versionID int) (json.RawMessage, bool) {
	var payload json.RawMessage
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT payload FROM project_config_values
		WHERE project_id = $1 AND environment_id = $2 AND version_id = $3
	`, projectID, envID, versionID).Scan(&payload)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "values version not found", "not_found")
		return nil, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return nil, false
	}
	return payload, true
}

func (h *DeploymentHandler) fetchPinnedGlobalValues(w http.ResponseWriter, r *http.Request, versions map[string]int) (map[string]map[string]any, map[string]json.RawMessage, bool) {
	gvMap := make(map[string]map[string]any, len(versions))
	gvPayloads := make(map[string]json.RawMessage, len(versions))

	for name, ver := range versions {
		var payload json.RawMessage
		err := h.DB.QueryRowContext(r.Context(), `
			SELECT payload FROM global_values
			WHERE name = $1 AND version_id = $2
		`, name, ver).Scan(&payload)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "global values "+name+" version not found", "not_found")
			return nil, nil, false
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return nil, nil, false
		}

		var flat map[string]any
		if err := json.Unmarshal(payload, &flat); err != nil {
			writeError(w, http.StatusInternalServerError, "invalid global values payload", "internal")
			return nil, nil, false
		}

		gvMap[name] = flat
		gvPayloads[name] = payload
	}

	return gvMap, gvPayloads, true
}

type prevTemplateData struct {
	RenderedOutput *string
	TemplateBody   *string
}

func (h *DeploymentHandler) fetchLastDeploymentBaseline(r *http.Request, projectID, envID int64) (map[string]*prevTemplateData, *json.RawMessage, map[string]json.RawMessage) {
	// Find last successful deployment
	var depID int64
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT id FROM deployments
		WHERE project_id = $1 AND environment_id = $2 AND status = 'succeeded'
		ORDER BY created_at DESC LIMIT 1
	`, projectID, envID).Scan(&depID)
	if err != nil {
		return nil, nil, nil
	}

	// Fetch entries
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT de.template_name, de.rendered_output, de.values_version_id, pct.body
		FROM deployment_entries de
		LEFT JOIN project_config_templates pct
			ON pct.project_id = $2 AND pct.template_name = de.template_name AND pct.version_id = de.template_version_id
		WHERE de.deployment_id = $1
	`, depID, projectID)
	if err != nil {
		return nil, nil, nil
	}
	defer rows.Close()

	prevTemplates := map[string]*prevTemplateData{}
	var prevValuesVersionID int
	for rows.Next() {
		var name, rendered string
		var valsVer int
		var body *string
		if err := rows.Scan(&name, &rendered, &valsVer, &body); err != nil {
			continue
		}
		prevTemplates[name] = &prevTemplateData{
			RenderedOutput: &rendered,
			TemplateBody:   body,
		}
		prevValuesVersionID = valsVer
	}

	// Fetch previous values payload
	var prevValues *json.RawMessage
	if prevValuesVersionID > 0 {
		var payload json.RawMessage
		err = h.DB.QueryRowContext(r.Context(), `
			SELECT payload FROM project_config_values
			WHERE project_id = $1 AND environment_id = $2 AND version_id = $3
		`, projectID, envID, prevValuesVersionID).Scan(&payload)
		if err == nil {
			prevValues = &payload
		}
	}

	// Fetch previous global values
	gvRows, err := h.DB.QueryContext(r.Context(), `
		SELECT DISTINCT degr.global_values_name, degr.global_values_version_id
		FROM deployment_entry_global_refs degr
		JOIN deployment_entries de ON de.id = degr.deployment_entry_id
		WHERE de.deployment_id = $1
	`, depID)
	if err != nil {
		return prevTemplates, prevValues, nil
	}
	defer gvRows.Close()

	prevGlobalValues := map[string]json.RawMessage{}
	for gvRows.Next() {
		var name string
		var ver int
		if err := gvRows.Scan(&name, &ver); err != nil {
			continue
		}
		var payload json.RawMessage
		err = h.DB.QueryRowContext(r.Context(), `
			SELECT payload FROM global_values WHERE name = $1 AND version_id = $2
		`, name, ver).Scan(&payload)
		if err == nil {
			prevGlobalValues[name] = payload
		}
	}

	return prevTemplates, prevValues, prevGlobalValues
}
