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

	var problems []models.WorkspaceProblem

	// Extract required variables per template up front. Templates that fail to
	// parse are reported once here and excluded from the per-environment render
	// so the parse error is not repeated for every environment.
	type tmplInfo struct {
		body     string
		required []string
	}
	parsed := make(map[string]tmplInfo, len(templates))
	var renderInputs []services.TemplateInput
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

	// A template with required variables needs at least one environment to fill
	// them. Templates that need no values (no required variables) render fine
	// without environments, so their absence is not a problem on its own.
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
		payload := json.RawMessage("{}")
		if resp, state, verr := h.effectiveValues(ctx, projectID, userID, env.Name); verr != nil {
			return nil, verr
		} else if state == valuesPresent && len(resp.Payload) > 0 {
			payload = resp.Payload
		}

		// (1) Required-variable coverage: which top-level keys does this env supply?
		valueKeys := map[string]bool{}
		var asObject map[string]json.RawMessage
		if err := json.Unmarshal(payload, &asObject); err == nil {
			for k := range asObject {
				valueKeys[k] = true
			}
		}
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
			if len(missing) > 0 {
				sort.Strings(missing)
				problems = append(problems, models.WorkspaceProblem{
					Kind:            string(services.ErrMissingValues),
					TemplateName:    t.TemplateName,
					EnvironmentName: env.Name,
					MissingKeys:     missing,
					Message: fmt.Sprintf("environment %q is missing values for template %q: %v",
						env.Name, t.TemplateName, missing),
				})
			}
		}

		// (2) Full render: catches template execution errors and unresolvable
		// ${name.key} global-values references that the coverage check cannot see.
		refs, _ := services.ExtractGlobalValueRefs(payload)
		gvMap, err := h.latestGlobalValuesMap(ctx, refs)
		if err != nil {
			return nil, err
		}
		for _, rr := range services.RenderAll(renderInputs, payload, gvMap) {
			if rr.Error == nil {
				continue
			}
			problems = append(problems, models.WorkspaceProblem{
				Kind:            string(rr.Error.Kind),
				TemplateName:    rr.Error.TemplateName,
				EnvironmentName: env.Name,
				Message:         fmt.Sprintf("environment %q: %s", env.Name, rr.Error.Message),
			})
		}
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
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	problems, err := h.validateWorkspace(r.Context(), projectID, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
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
