package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"text/template"
	"text/template/parse"

	"github.com/Masterminds/sprig/v3"
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

// resolveLatestProjectVersion returns (id, version_id) of the latest non-anchor
// project_version for the given project, or sql.ErrNoRows when the project has
// none. Anchor rows are skipped because they only exist to back-fill old
// deployments and never represent a head editable state.
func resolveLatestProjectVersion(r *http.Request, db *sql.DB, projectID int64) (int64, int, error) {
	var id int64
	var versionID int
	err := db.QueryRowContext(r.Context(), `
		SELECT id, version_id FROM project_versions
		WHERE project_id = $1 AND NOT is_anchor
		ORDER BY version_id DESC LIMIT 1
	`, projectID).Scan(&id, &versionID)
	return id, versionID, err
}

// GetLatest returns the body of a template as of the project's latest version.
func (h *TemplateHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	templateName := chi.URLParam(r, "templateName")

	var tmpl models.ProjectConfigTemplate
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT t.id, t.project_id, t.template_name, t.version_id, t.body, t.commit_message, t.created_by, t.created_at
		FROM project_version_templates pvt
		JOIN project_config_templates t ON t.id = pvt.template_row_id
		WHERE pvt.template_name = $2
		  AND pvt.project_version_id = (
		      SELECT id FROM project_versions
		      WHERE project_id = $1 AND NOT is_anchor
		      ORDER BY version_id DESC LIMIT 1
		  )
	`, projectID, templateName).Scan(
		&tmpl.ID, &tmpl.ProjectID, &tmpl.TemplateName, &tmpl.VersionID,
		&tmpl.Body, &tmpl.CommitMessage, &tmpl.CreatedBy, &tmpl.CreatedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "template not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, tmpl)
}

// GetVersion returns the body of a template pinned to a specific project
// version. The URL's versionID is the per-project ordinal.
func (h *TemplateHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
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
		SELECT t.id, t.project_id, t.template_name, t.version_id, t.body, t.commit_message, t.created_by, t.created_at
		FROM project_versions pv
		JOIN project_version_templates pvt ON pvt.project_version_id = pv.id
		JOIN project_config_templates t ON t.id = pvt.template_row_id
		WHERE pv.project_id = $1 AND pv.version_id = $2 AND pvt.template_name = $3
	`, projectID, versionID, templateName).Scan(
		&tmpl.ID, &tmpl.ProjectID, &tmpl.TemplateName, &tmpl.VersionID,
		&tmpl.Body, &tmpl.CommitMessage, &tmpl.CreatedBy, &tmpl.CreatedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "template version not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, tmpl)
}

// ListForProject returns every template visible at the project's latest version.
func (h *TemplateHandler) ListForProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	pvID, _, err := resolveLatestProjectVersion(r, h.DB, projectID)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, models.ListResponse[models.ProjectConfigTemplate]{Items: []models.ProjectConfigTemplate{}, Count: 0})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT t.id, t.project_id, t.template_name, t.version_id, t.body, t.commit_message, t.created_by, t.created_at
		FROM project_version_templates pvt
		JOIN project_config_templates t ON t.id = pvt.template_row_id
		WHERE pvt.project_version_id = $1
		ORDER BY t.template_name
	`, pvID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer rows.Close()

	templates := []models.ProjectConfigTemplate{}
	for rows.Next() {
		var t models.ProjectConfigTemplate
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.TemplateName, &t.VersionID, &t.Body, &t.CommitMessage, &t.CreatedBy, &t.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.ProjectConfigTemplate]{Items: templates, Count: len(templates)})
}

// ListVersions returns the project versions in which this template's body
// changed (i.e. the entries where template_row_id differs from the immediate
// predecessor version's row for the same name).
func (h *TemplateHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	templateName := chi.URLParam(r, "templateName")

	rows, err := h.DB.QueryContext(r.Context(), `
		WITH walk AS (
			SELECT pv.version_id AS pv_version_id, pvt.template_row_id,
			       t.id, t.project_id, t.template_name, t.version_id, t.body,
			       t.commit_message, t.created_by, t.created_at,
			       LAG(pvt.template_row_id) OVER (ORDER BY pv.version_id) AS prev_row_id
			FROM project_versions pv
			JOIN project_version_templates pvt ON pvt.project_version_id = pv.id
			JOIN project_config_templates t ON t.id = pvt.template_row_id
			WHERE pv.project_id = $1 AND pvt.template_name = $2 AND NOT pv.is_anchor
		)
		SELECT id, project_id, template_name, version_id, body, commit_message, created_by, created_at
		FROM walk
		WHERE prev_row_id IS DISTINCT FROM template_row_id
		ORDER BY pv_version_id DESC
	`, projectID, templateName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer rows.Close()

	versions := []models.ProjectConfigTemplate{}
	for rows.Next() {
		var t models.ProjectConfigTemplate
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.TemplateName, &t.VersionID, &t.Body, &t.CommitMessage, &t.CreatedBy, &t.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		versions = append(versions, t)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.ProjectConfigTemplate]{Items: versions, Count: len(versions)})
}

// Variables extracts template variables (and their defaults) from the latest template body.
func (h *TemplateHandler) Variables(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	templateName := chi.URLParam(r, "templateName")

	var body string
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT t.body
		FROM project_versions pv
		JOIN project_version_templates pvt ON pvt.project_version_id = pv.id
		JOIN project_config_templates t ON t.id = pvt.template_row_id
		WHERE pv.project_id = $1 AND pvt.template_name = $2 AND NOT pv.is_anchor
		ORDER BY pv.version_id DESC LIMIT 1
	`, projectID, templateName).Scan(&body)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "template not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	vars, err := extractTemplateVariables(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse template: %v", err), "parse_error")
		return
	}

	writeJSON(w, http.StatusOK, models.TemplateVariablesResponse{Variables: vars})
}

// ProjectVariables returns the union of variables across all templates in a project.
func (h *TemplateHandler) ProjectVariables(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	pvID, _, err := resolveLatestProjectVersion(r, h.DB, projectID)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, models.TemplateVariablesResponse{Variables: []models.TemplateVariable{}})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT t.body
		FROM project_version_templates pvt
		JOIN project_config_templates t ON t.id = pvt.template_row_id
		WHERE pvt.project_version_id = $1
	`, pvID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer rows.Close()

	seen := map[string]*models.TemplateVariable{}
	var order []string

	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		vars, err := extractTemplateVariables(body)
		if err != nil {
			continue // skip unparseable templates
		}
		for _, v := range vars {
			existing, exists := seen[v.Name]
			if !exists {
				vCopy := v
				seen[v.Name] = &vCopy
				order = append(order, v.Name)
				continue
			}
			// Same variable used in another template: reconcile the kind
			// (differing kinds => conflict) and keep the first default seen.
			existing.Kind = mergeVarKind(existing.Kind, v.Kind)
			if existing.Default == nil && v.Default != nil {
				existing.Default = v.Default
			}
		}
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	result := make([]models.TemplateVariable, 0, len(order))
	for _, name := range order {
		result = append(result, *seen[name])
	}

	writeJSON(w, http.StatusOK, models.TemplateVariablesResponse{Variables: result})
}

// extractTemplateVariables parses a Go template body with Sprig functions and
// walks the AST to find top-level field references ({{ .name }}) and their
// default values ({{ .name | default "value" }}).
func extractTemplateVariables(body string) ([]models.TemplateVariable, error) {
	tmpl, err := template.New("").Funcs(sprig.TxtFuncMap()).Parse(body)
	if err != nil {
		return nil, err
	}

	usage := map[string]*varUsage{}
	var order []string

	for _, node := range tmpl.Root.Nodes {
		walkNode(node, usage, &order)
	}

	vars := make([]models.TemplateVariable, 0, len(order))
	for _, name := range order {
		u := usage[name]
		vars = append(vars, models.TemplateVariable{Name: name, Default: u.defaultVal, Kind: u.kind()})
	}
	return vars, nil
}

// varUsage accumulates how a single variable name is referenced while walking a
// template AST. asList is set when the name is the target of a {{range}};
// asScalar is set for any other reference. A name flagged as both yields a
// "conflict" kind.
type varUsage struct {
	defaultVal *string
	asList     bool
	asScalar   bool
}

func (u *varUsage) kind() string {
	if u.asList && u.asScalar {
		return varKindConflict
	}
	if u.asList {
		return varKindList
	}
	return varKindString
}

const (
	varKindString   = "string"
	varKindList     = "list"
	varKindConflict = "conflict"
)

// mergeVarKind combines the kind of the same variable seen in two templates.
// Matching kinds are kept as-is; any disagreement (e.g. list vs string, or an
// existing conflict) collapses to "conflict".
func mergeVarKind(a, b string) string {
	if a == b {
		return a
	}
	return varKindConflict
}

func walkNode(node parse.Node, usage map[string]*varUsage, order *[]string) {
	switch n := node.(type) {
	case *parse.ActionNode:
		extractFromPipe(n.Pipe, false, usage, order)
	case *parse.IfNode:
		walkBranchNode(n.Pipe, n.List, n.ElseList, false, usage, order)
	case *parse.RangeNode:
		// The ranged expression is a list; its body is walked as scalars.
		walkBranchNode(n.Pipe, n.List, n.ElseList, true, usage, order)
	case *parse.WithNode:
		walkBranchNode(n.Pipe, n.List, n.ElseList, false, usage, order)
	case *parse.ListNode:
		walkList(n, usage, order)
	}
}

// walkBranchNode handles the if/range/with shape: extract the pipe (as a list
// for range, scalar otherwise) then recurse into both branches.
func walkBranchNode(pipe *parse.PipeNode, list, elseList *parse.ListNode, pipeIsList bool, usage map[string]*varUsage, order *[]string) {
	extractFromPipe(pipe, pipeIsList, usage, order)
	walkList(list, usage, order)
	walkList(elseList, usage, order)
}

// walkList recurses into every child of a (possibly nil) ListNode.
func walkList(list *parse.ListNode, usage map[string]*varUsage, order *[]string) {
	if list == nil {
		return
	}
	for _, child := range list.Nodes {
		walkNode(child, usage, order)
	}
}

func extractFromPipe(pipe *parse.PipeNode, asList bool, usage map[string]*varUsage, order *[]string) {
	if pipe == nil {
		return
	}

	var fieldName string
	var defaultVal *string

	for _, cmd := range pipe.Cmds {
		if name := extractFieldFromCmd(cmd); name != "" {
			fieldName = name
		}
		if val := extractDefaultFromCmd(cmd); val != nil {
			defaultVal = val
		}
	}

	if fieldName == "" {
		return
	}
	u, exists := usage[fieldName]
	if !exists {
		u = &varUsage{}
		usage[fieldName] = u
		*order = append(*order, fieldName)
	}
	if asList {
		u.asList = true
	} else {
		u.asScalar = true
	}
	if defaultVal != nil && u.defaultVal == nil {
		u.defaultVal = defaultVal
	}
}

// extractFieldFromCmd returns the last FieldNode identifier found in the
// command's args (matching the original last-wins behaviour), or "" if none.
func extractFieldFromCmd(cmd *parse.CommandNode) string {
	var name string
	for _, arg := range cmd.Args {
		field, ok := arg.(*parse.FieldNode)
		if !ok || len(field.Ident) == 0 {
			continue
		}
		name = field.Ident[0]
	}
	return name
}

// extractDefaultFromCmd returns the literal arg of a `default "x"` call, or
// nil if the command is not a recognised default call.
func extractDefaultFromCmd(cmd *parse.CommandNode) *string {
	if len(cmd.Args) < 2 {
		return nil
	}
	ident, ok := cmd.Args[0].(*parse.IdentifierNode)
	if !ok || ident.Ident != "default" {
		return nil
	}
	val := extractLiteral(cmd.Args[1])
	if val == "" {
		return nil
	}
	return &val
}

func extractLiteral(node parse.Node) string {
	switch n := node.(type) {
	case *parse.StringNode:
		return n.Text
	case *parse.NumberNode:
		return n.Text
	case *parse.BoolNode:
		if n.True {
			return "true"
		}
		return "false"
	}
	return ""
}
