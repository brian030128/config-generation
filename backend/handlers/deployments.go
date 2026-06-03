package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/brian/config-generation/backend/models"
	"github.com/brian/config-generation/backend/services"
	"github.com/go-chi/chi/v5"
)

type DeploymentHandler struct {
	DB *sql.DB
}

// Preview renders all templates pinned by the project version (filtered to the
// target environment's values slice) plus the supplied group versions.
func (h *DeploymentHandler) Preview(w http.ResponseWriter, r *http.Request) {
	projectID, envID, ok := h.resolveProjectEnv(w, r)
	if !ok {
		return
	}

	var req models.DeployPreviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}

	templates, templateBodies, ok := h.fetchPinnedTemplates(w, r, projectID, req.ProjectVersionID)
	if !ok {
		return
	}
	valuesPayload, ok := h.fetchPinnedValues(w, r, projectID, envID, req.ProjectVersionID)
	if !ok {
		return
	}
	gvMap, gvPayloads, ok := h.fetchPinnedGlobalValues(w, r, req.GroupVersionIDsRaw)
	if !ok {
		return
	}

	renderResults := services.RenderAll(templates, valuesPayload, gvMap)

	prevTemplates, prevValues, prevGlobalValues, prevDeployment := h.fetchLastDeploymentBaseline(r, projectID, envID)

	hasErrors := false
	results := make([]models.TemplateRenderResult, len(renderResults))
	for i, rr := range renderResults {
		results[i] = models.TemplateRenderResult{
			TemplateName: rr.TemplateName,
			TemplateBody: templateBodies[rr.TemplateName],
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

	writeJSON(w, http.StatusOK, models.DeployPreviewResponse{
		Results:              results,
		ValuesPayload:        valuesPayload,
		PreviousValues:       prevValues,
		ProjectVersionID:     req.ProjectVersionID,
		GlobalValues:         gvPayloads,
		PreviousGlobalValues: prevGlobalValues,
		GroupVersions:        req.GroupVersionIDsRaw,
		HasErrors:            hasErrors,
		PreviousDeployment:   prevDeployment,
	})
}

// Execute renders all templates and records a deployment pinning one
// project_version + one group_version per referenced group.
func (h *DeploymentHandler) Execute(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projectID, envID, ok := h.resolveProjectEnv(w, r)
	if !ok {
		return
	}

	var req models.DeployRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}

	templates, _, ok := h.fetchPinnedTemplates(w, r, projectID, req.ProjectVersionID)
	if !ok {
		return
	}
	valuesPayload, ok := h.fetchPinnedValues(w, r, projectID, envID, req.ProjectVersionID)
	if !ok {
		return
	}
	gvMap, _, ok := h.fetchPinnedGlobalValues(w, r, req.GroupVersionIDsRaw)
	if !ok {
		return
	}

	renderResults := services.RenderAll(templates, valuesPayload, gvMap)

	for _, rr := range renderResults {
		if rr.Error != nil {
			writeError(w, http.StatusUnprocessableEntity, rr.Error.Message, string(rr.Error.Kind))
			return
		}
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tx.Rollback()

	var deploymentID int64
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO deployments (project_id, environment_id, project_version_id, status, commit_message, created_by)
		VALUES ($1, $2, $3, 'succeeded', $4, $5)
		RETURNING id
	`, projectID, envID, req.ProjectVersionID, req.CommitMessage, user.UserID).Scan(&deploymentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create deployment", "internal")
		return
	}

	if err := h.insertDeploymentEntries(r.Context(), tx, deploymentID, renderResults); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "internal")
		return
	}
	if err := h.insertDeploymentGroupRefs(r.Context(), tx, deploymentID, req.GroupVersionIDsRaw); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit deployment", "internal")
		return
	}

	results := make([]models.TemplateRenderResult, len(renderResults))
	for i, rr := range renderResults {
		results[i] = models.TemplateRenderResult{TemplateName: rr.TemplateName, RenderedOutput: rr.RenderedOutput}
	}
	writeJSON(w, http.StatusCreated, models.DeployResponse{
		DeploymentID: deploymentID, Status: "succeeded", Results: results,
	})
}

// GetLatest returns the version set pinned by the most recent successful
// deployment.
func (h *DeploymentHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	projectID, envID, ok := h.resolveProjectEnv(w, r)
	if !ok {
		return
	}

	var dep models.Deployment
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT id, project_version_id, commit_message, created_at
		FROM deployments
		WHERE project_id = $1 AND environment_id = $2 AND status = 'succeeded'
		ORDER BY created_at DESC LIMIT 1
	`, projectID, envID).Scan(&dep.ID, &dep.ProjectVersionID, &dep.CommitMessage, &dep.CreatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "no successful deployment found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	groupVersions, err := h.loadDeploymentGroupVersions(r.Context(), dep.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, models.LatestDeploymentResponse{
		DeploymentID:     dep.ID,
		ProjectVersionID: dep.ProjectVersionID,
		GroupVersions:    groupVersions,
		CreatedAt:        dep.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		CommitMessage:    dep.CommitMessage,
	})
}

// --- helpers ---

func (h *DeploymentHandler) insertDeploymentEntries(ctx context.Context, tx *sql.Tx, deploymentID int64, renderResults []services.RenderResult) error {
	for _, rr := range renderResults {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO deployment_entries (deployment_id, template_name, rendered_output)
			VALUES ($1, $2, $3)
		`, deploymentID, rr.TemplateName, *rr.RenderedOutput); err != nil {
			return fmt.Errorf("failed to create deployment entry")
		}
	}
	return nil
}

func (h *DeploymentHandler) insertDeploymentGroupRefs(ctx context.Context, tx *sql.Tx, deploymentID int64, groupVersionsByName map[string]int) error {
	for groupName, versionOrd := range groupVersionsByName {
		var groupID, groupVersionID int64
		err := tx.QueryRowContext(ctx, `
			SELECT g.id, ggv.id
			FROM global_values_groups g
			JOIN global_values_group_versions ggv ON ggv.group_id = g.id AND ggv.version_id = $2
			WHERE g.name = $1
		`, groupName, versionOrd).Scan(&groupID, &groupVersionID)
		if err != nil {
			return fmt.Errorf("group %q version %d not found", groupName, versionOrd)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO deployment_group_refs (deployment_id, group_id, group_version_id)
			VALUES ($1, $2, $3)
		`, deploymentID, groupID, groupVersionID); err != nil {
			return fmt.Errorf("failed to create deployment group ref")
		}
	}
	return nil
}

func (h *DeploymentHandler) loadDeploymentGroupVersions(ctx context.Context, deploymentID int64) (map[string]int, error) {
	rows, err := h.DB.QueryContext(ctx, `
		SELECT g.name, ggv.version_id
		FROM deployment_group_refs dgr
		JOIN global_values_groups g ON g.id = dgr.group_id
		JOIN global_values_group_versions ggv ON ggv.id = dgr.group_version_id
		WHERE dgr.deployment_id = $1
	`, deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var name string
		var ord int
		if err := rows.Scan(&name, &ord); err != nil {
			return nil, err
		}
		out[name] = ord
	}
	return out, rows.Err()
}

func (h *DeploymentHandler) resolveProjectEnv(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return 0, 0, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return 0, 0, false
	}

	envName := chi.URLParam(r, "envName")
	envID, err := resolveEnvironmentID(r, h.DB, projectID, envName)
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

// fetchPinnedTemplates returns every template snapshotted by the project
// version. The set is fixed by the snapshot — callers do not get to pick a
// subset.
func (h *DeploymentHandler) fetchPinnedTemplates(w http.ResponseWriter, r *http.Request, projectID, projectVersionID int64) ([]services.TemplateInput, map[string]string, bool) {
	if err := h.assertProjectVersionBelongsToProject(r.Context(), projectID, projectVersionID); err != nil {
		writeError(w, http.StatusNotFound, "project version not found for project", "not_found")
		return nil, nil, false
	}
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT pvt.template_name, t.body
		FROM project_version_templates pvt
		JOIN project_config_templates t ON t.id = pvt.template_row_id
		WHERE pvt.project_version_id = $1
	`, projectVersionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return nil, nil, false
	}
	defer rows.Close()

	templates := []services.TemplateInput{}
	bodies := map[string]string{}
	for rows.Next() {
		var name, body string
		if err := rows.Scan(&name, &body); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return nil, nil, false
		}
		templates = append(templates, services.TemplateInput{Name: name, Body: body})
		bodies[name] = body
	}
	return templates, bodies, true
}

func (h *DeploymentHandler) fetchPinnedValues(w http.ResponseWriter, r *http.Request, projectID, envID, projectVersionID int64) (json.RawMessage, bool) {
	var payload json.RawMessage
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT v.payload
		FROM project_version_values pvv
		JOIN project_config_values v ON v.id = pvv.values_row_id
		JOIN project_versions pv ON pv.id = pvv.project_version_id
		WHERE pv.id = $1 AND pv.project_id = $2 AND pvv.environment_id = $3
	`, projectVersionID, projectID, envID).Scan(&payload)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "values not found at project version for this environment", "not_found")
		return nil, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return nil, false
	}
	return payload, true
}

func (h *DeploymentHandler) fetchPinnedGlobalValues(w http.ResponseWriter, r *http.Request, groupVersionsByName map[string]int) (map[string]map[string]any, map[string]json.RawMessage, bool) {
	gvMap := make(map[string]map[string]any)
	gvPayloads := make(map[string]json.RawMessage)

	for groupName, versionOrd := range groupVersionsByName {
		rows, err := h.DB.QueryContext(r.Context(), `
			SELECT gv.name, gv.payload
			FROM global_values_groups g
			JOIN global_values_group_versions ggv ON ggv.group_id = g.id
			JOIN global_values_group_version_entries e ON e.group_version_id = ggv.id
			JOIN global_values gv ON gv.id = e.gv_row_id
			WHERE g.name = $1 AND ggv.version_id = $2
		`, groupName, versionOrd)
		if err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return nil, nil, false
		}
		flat := map[string]any{}
		groupPayload := map[string]any{}
		any_ := false
		for rows.Next() {
			var name string
			var payload json.RawMessage
			if err := rows.Scan(&name, &payload); err != nil {
				rows.Close()
				writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
				return nil, nil, false
			}
			var parsed map[string]any
			if err := json.Unmarshal(payload, &parsed); err != nil {
				rows.Close()
				writeError(w, http.StatusInternalServerError, "invalid global values payload", "internal")
				return nil, nil, false
			}
			for k, v := range parsed {
				flat[k] = v
			}
			groupPayload[name] = parsed
			any_ = true
		}
		rows.Close()
		if !any_ {
			writeError(w, http.StatusNotFound, "global values group "+groupName+" version not found", "not_found")
			return nil, nil, false
		}
		gvMap[groupName] = flat
		bs, _ := json.Marshal(groupPayload)
		gvPayloads[groupName] = bs
	}

	return gvMap, gvPayloads, true
}

func (h *DeploymentHandler) assertProjectVersionBelongsToProject(ctx context.Context, projectID, projectVersionID int64) error {
	var ok bool
	err := h.DB.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM project_versions WHERE id = $1 AND project_id = $2)
	`, projectVersionID, projectID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return sql.ErrNoRows
	}
	return nil
}

type prevTemplateData struct {
	RenderedOutput *string
	TemplateBody   *string
}

func (h *DeploymentHandler) fetchLastDeploymentBaseline(r *http.Request, projectID, envID int64) (map[string]*prevTemplateData, *json.RawMessage, map[string]json.RawMessage, *models.PreviousDeploymentInfo) {
	var depID, prevPVID int64
	info := &models.PreviousDeploymentInfo{}
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT d.id, d.project_version_id, d.created_at, pv.version_id
		FROM deployments d
		JOIN project_versions pv ON pv.id = d.project_version_id
		WHERE d.project_id = $1 AND d.environment_id = $2 AND d.status = 'succeeded'
		ORDER BY d.created_at DESC LIMIT 1
	`, projectID, envID).Scan(&depID, &prevPVID, &info.CreatedAt, &info.ProjectVersion)
	if err != nil {
		return nil, nil, nil, nil
	}
	info.DeploymentID = depID

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT de.template_name, de.rendered_output, t.body
		FROM deployment_entries de
		LEFT JOIN project_version_templates pvt ON pvt.project_version_id = $2 AND pvt.template_name = de.template_name
		LEFT JOIN project_config_templates t ON t.id = pvt.template_row_id
		WHERE de.deployment_id = $1
	`, depID, prevPVID)
	if err != nil {
		return nil, nil, nil, info
	}
	defer rows.Close()

	prevTemplates := map[string]*prevTemplateData{}
	for rows.Next() {
		var name, rendered string
		var body *string
		if err := rows.Scan(&name, &rendered, &body); err != nil {
			continue
		}
		prevTemplates[name] = &prevTemplateData{RenderedOutput: &rendered, TemplateBody: body}
	}

	var prevValues *json.RawMessage
	var payload json.RawMessage
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT v.payload
		FROM project_version_values pvv
		JOIN project_config_values v ON v.id = pvv.values_row_id
		WHERE pvv.project_version_id = $1 AND pvv.environment_id = $2
	`, prevPVID, envID).Scan(&payload)
	if err == nil {
		prevValues = &payload
	}

	gvRows, err := h.DB.QueryContext(r.Context(), `
		SELECT g.name, ggv.id
		FROM deployment_group_refs dgr
		JOIN global_values_groups g ON g.id = dgr.group_id
		JOIN global_values_group_versions ggv ON ggv.id = dgr.group_version_id
		WHERE dgr.deployment_id = $1
	`, depID)
	if err != nil {
		return prevTemplates, prevValues, nil, info
	}
	defer gvRows.Close()

	prevGlobalValues := map[string]json.RawMessage{}
	for gvRows.Next() {
		var groupName string
		var groupVersionID int64
		if err := gvRows.Scan(&groupName, &groupVersionID); err != nil {
			continue
		}
		values, err := loadGroupVersionValues(r.Context(), h.DB, groupVersionID)
		if err == nil {
			groupPayload := map[string]any{}
			for name, payload := range values {
				var parsed any
				_ = json.Unmarshal(payload, &parsed)
				groupPayload[name] = parsed
			}
			bs, _ := json.Marshal(groupPayload)
			prevGlobalValues[groupName] = bs
		}
	}

	return prevTemplates, prevValues, prevGlobalValues, info
}
