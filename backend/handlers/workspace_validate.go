package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/brian/config-generation/backend/models"
	"github.com/brian/config-generation/backend/services"
	"github.com/go-chi/chi/v5"
	"net/http"
)

// tmplInfo holds the parsed body and required-variable list for a workspace
// template; computed once and reused per environment.
type tmplInfo struct {
	body     string
	required []string
}

// parseWorkspaceTemplates extracts required-variable info for each template.
// Templates that fail to parse are reported as problems and excluded from the
// returned render inputs so the parse error is not repeated per environment.
func parseWorkspaceTemplates(templates []models.WorkspaceTemplateItem) (map[string]tmplInfo, []services.TemplateInput, bool, []models.WorkspaceProblem) {
	parsed := make(map[string]tmplInfo, len(templates))
	var renderInputs []services.TemplateInput
	var problems []models.WorkspaceProblem
	anyRequired := false
	for _, t := range templates {
		vars, perr := extractTemplateVariables(t.Body)
		if perr != nil {
			problems = append(problems, models.WorkspaceProblem{
				Kind:         string(services.ErrTemplateParse),
				TemplateName: t.TemplateName,
				Message:      fmt.Sprintf("template %q failed to parse: %s", t.TemplateName, perr),
			})
			continue
		}
		var required []string
		for _, v := range vars {
			if v.Default == nil {
				required = append(required, v.Name)
			}
		}
		if len(required) > 0 {
			anyRequired = true
		}
		parsed[t.TemplateName] = tmplInfo{body: t.Body, required: required}
		renderInputs = append(renderInputs, services.TemplateInput{Name: t.TemplateName, Body: t.Body})
	}
	return parsed, renderInputs, anyRequired, problems
}

// envValueKeys returns the set of top-level keys provided by an env's values
// payload. Non-object payloads contribute no keys.
func envValueKeys(payload json.RawMessage) map[string]bool {
	keys := map[string]bool{}
	var asObject map[string]json.RawMessage
	if err := json.Unmarshal(payload, &asObject); err == nil {
		for k := range asObject {
			keys[k] = true
		}
	}
	return keys
}

// checkRequiredCoverage reports a problem for each parsed template whose
// required keys are not all supplied by the env.
func checkRequiredCoverage(templates []models.WorkspaceTemplateItem, parsed map[string]tmplInfo, valueKeys map[string]bool, envName string) []models.WorkspaceProblem {
	var problems []models.WorkspaceProblem
	for _, t := range templates {
		info, ok := parsed[t.TemplateName]
		if !ok {
			continue // unparseable; already reported
		}
		var missing []string
		for _, key := range info.required {
			if !valueKeys[key] {
				missing = append(missing, key)
			}
		}
		if len(missing) == 0 {
			continue
		}
		sort.Strings(missing)
		problems = append(problems, models.WorkspaceProblem{
			Kind:            string(services.ErrMissingValues),
			TemplateName:    t.TemplateName,
			EnvironmentName: envName,
			MissingKeys:     missing,
			Message: fmt.Sprintf("environment %q is missing values for template %q: %v",
				envName, t.TemplateName, missing),
		})
	}
	return problems
}

// renderProblemsForEnv runs the full render against an environment and
// returns one problem per render error (template execution, unresolved global
// values, etc.).
func (h *PullRequestHandler) renderProblemsForEnv(ctx context.Context, renderInputs []services.TemplateInput, payload json.RawMessage, envName string) ([]models.WorkspaceProblem, error) {
	refs, _ := services.ExtractGlobalValueRefs(payload)
	gvMap, err := h.latestGlobalValuesMap(ctx, refs)
	if err != nil {
		return nil, err
	}
	var problems []models.WorkspaceProblem
	for _, rr := range services.RenderAll(renderInputs, payload, gvMap) {
		if rr.Error == nil {
			continue
		}
		problems = append(problems, models.WorkspaceProblem{
			Kind:            string(rr.Error.Kind),
			TemplateName:    rr.Error.TemplateName,
			EnvironmentName: envName,
			Message:         fmt.Sprintf("environment %q: %s", envName, rr.Error.Message),
		})
	}
	return problems, nil
}

// envPayloadForValidation returns the env's effective values payload, falling
// back to "{}" when the env has no values set.
func (h *PullRequestHandler) envPayloadForValidation(ctx context.Context, projectID, userID int64, envName string) (json.RawMessage, error) {
	resp, state, err := h.effectiveValues(ctx, projectID, userID, envName)
	if err != nil {
		return nil, err
	}
	if state == valuesPresent && len(resp.Payload) > 0 {
		return resp.Payload, nil
	}
	return json.RawMessage("{}"), nil
}

// validateWorkspace checks whether the caller's workspace for a project would
// produce valid configs. It returns one problem per template/environment pair
// that cannot render. A workspace with no problems is safe to submit.
//
// Two independent checks run because the render engine does not set
// missingkey=error: a template referencing an absent value renders as
// "<no value>" rather than failing. So we (1) compare each template's required
// variables (those without a Go-template default) against the keys each
// environment supplies, and (2) run a full render to catch parse/exec errors
// and unresolvable ${name.key} global-values references.
func (h *PullRequestHandler) validateWorkspace(ctx context.Context, projectID, userID int64) ([]models.WorkspaceProblem, error) {
	templates, err := h.effectiveTemplates(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	environments, err := h.effectiveEnvironments(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return nil, nil // nothing to render
	}

	parsed, renderInputs, anyRequired, problems := parseWorkspaceTemplates(templates)

	// A template with required variables needs at least one environment to fill
	// them. Templates that need no values render fine without environments.
	if len(environments) == 0 {
		if anyRequired {
			problems = append(problems, models.WorkspaceProblem{
				Kind:    "no_environments",
				Message: "the workspace has templates with required values but no environments to provide them",
			})
		}
		return problems, nil
	}

	for _, env := range environments {
		payload, err := h.envPayloadForValidation(ctx, projectID, userID, env.Name)
		if err != nil {
			return nil, err
		}
		problems = append(problems, checkRequiredCoverage(templates, parsed, envValueKeys(payload), env.Name)...)
		envProblems, err := h.renderProblemsForEnv(ctx, renderInputs, payload, env.Name)
		if err != nil {
			return nil, err
		}
		problems = append(problems, envProblems...)
	}
	return problems, nil
}

// latestGlobalValuesMap loads the latest live version of each named global
// values entry into the map shape RenderAll expects. Missing or malformed
// entries are left out so RenderAll reports them as unresolved references.
func (h *PullRequestHandler) latestGlobalValuesMap(ctx context.Context, names []string) (map[string]map[string]any, error) {
	gvMap := make(map[string]map[string]any, len(names))
	for _, name := range names {
		var payload json.RawMessage
		err := h.DB.QueryRowContext(ctx, `
			SELECT payload FROM global_values WHERE name = $1 ORDER BY version_id DESC LIMIT 1
		`, name).Scan(&payload)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		var flat map[string]any
		if err := json.Unmarshal(payload, &flat); err != nil {
			continue
		}
		gvMap[name] = flat
	}
	return gvMap, nil
}

// ValidateWorkspace reports whether the caller's workspace can be submitted,
// returning the list of problems (empty when valid).
func (h *PullRequestHandler) ValidateWorkspace(w http.ResponseWriter, r *http.Request) {
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

	problems, err := h.validateWorkspace(r.Context(), projectID, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	if problems == nil {
		problems = []models.WorkspaceProblem{}
	}
	writeJSON(w, http.StatusOK, models.WorkspaceValidationResponse{
		Valid:    len(problems) == 0,
		Problems: problems,
	})
}
