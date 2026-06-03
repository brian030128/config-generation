package models

import (
	"encoding/json"
	"time"
)

type ListResponse[T any] struct {
	Items []T `json:"items"`
	Count int `json:"count"`
}

// ProjectMembersResponse is the project members list plus whether the caller
// holds grant(p) — i.e. may add/remove members. The frontend uses
// ViewerCanManage to show or hide the member-management controls.
type ProjectMembersResponse struct {
	Items           []ProjectMember `json:"items"`
	Count           int             `json:"count"`
	ViewerCanManage bool            `json:"viewer_can_manage"`
}

// RolesResponse is the roles list for a scope (project or global values entry)
// plus whether the caller holds grant on that scope — i.e. may create/edit/
// delete roles and assign members. Mirrors ProjectMembersResponse: the list is
// readable by anyone with read on the scope, while ViewerCanManage gates the
// management controls in the UI.
type RolesResponse struct {
	Items           []Role `json:"items"`
	Count           int    `json:"count"`
	ViewerCanManage bool   `json:"viewer_can_manage"`
}

// GlobalValuesGroupDetailResponse is the group's latest version (materialized
// values map) plus whether the caller holds grant on the group.
type GlobalValuesGroupDetailResponse struct {
	Group           GlobalValuesGroup        `json:"group"`
	LatestVersion   GlobalValuesGroupVersion `json:"latest_version"`
	ViewerCanManage bool                     `json:"viewer_can_manage"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// MeResponse is the authenticated user's own profile. It is the only endpoint
// that exposes the superuser flag, and only for the caller themselves — user
// listings/searches and role member listings deliberately omit it.
type MeResponse struct {
	User
	Superuser bool `json:"superuser"`
}

type AuthConfigResponse struct {
	OIDCEnabled          bool   `json:"oidc_enabled"`
	OIDCProviderName     string `json:"oidc_provider_name"`
	PasswordLoginEnabled bool   `json:"password_login_enabled"`
	RegistrationEnabled  bool   `json:"registration_enabled"`
}

type TemplateVariable struct {
	Name    string  `json:"name"`
	Default *string `json:"default,omitempty"`
	// Kind is how the variable is used in the template body: "string" for a
	// plain field reference, "list" when it is the target of a {{range}}, and
	// "conflict" when the same name is used both ways (in one template or
	// across the project's templates). The env-values UI renders a list editor
	// for "list" and surfaces an error for "conflict".
	Kind string `json:"kind"`
}

// Workspace overlay views: the published base merged with the caller's own
// staged changes. `staged` marks an item touched by the active workspace;
// `operation` is the staged operation (create|update|delete) when staged.

type WorkspaceTemplateItem struct {
	TemplateName   string `json:"template_name"`
	Body           string `json:"body"`
	BaseProjectVID int    `json:"base_project_version_id"` // latest project version this template is in; 0 if new in the workspace
	Staged         bool   `json:"staged"`
	Operation      string `json:"operation,omitempty"`
}

type WorkspaceEnvironmentItem struct {
	Name      string `json:"name"`
	Staged    bool   `json:"staged"`
	Operation string `json:"operation,omitempty"`
}

type WorkspaceValuesResponse struct {
	EnvironmentName string          `json:"environment_name"`
	Payload         json.RawMessage `json:"payload"`
	BaseProjectVID  int             `json:"base_project_version_id"` // latest project version this env is in; 0 if staged-new
	Staged          bool            `json:"staged"`
	Operation       string          `json:"operation,omitempty"`
}

type TemplateVariablesResponse struct {
	Variables []TemplateVariable `json:"variables"`
}

// WorkspaceProblem is a single reason a workspace cannot be submitted: a
// template/environment pair that fails to produce a valid config. Kind mirrors
// services.RenderErrorKind plus the workspace-specific "no_environments".
type WorkspaceProblem struct {
	Kind            string   `json:"kind"` // no_environments|missing_values|unknown_global_values|unknown_key|template_parse|template_exec
	TemplateName    string   `json:"template_name,omitempty"`
	EnvironmentName string   `json:"environment_name,omitempty"`
	MissingKeys     []string `json:"missing_keys,omitempty"`
	Message         string   `json:"message"`
}

// WorkspaceValidationResponse is returned by the workspace validate endpoint and
// embedded in the 422 body when a submit is blocked.
type WorkspaceValidationResponse struct {
	Valid    bool               `json:"valid"`
	Problems []WorkspaceProblem `json:"problems"`
}

type TemplateRenderResult struct {
	TemplateName         string  `json:"template_name"`
	RenderedOutput       *string `json:"rendered_output,omitempty"`
	Error                *string `json:"error,omitempty"`
	ErrorKind            *string `json:"error_kind,omitempty"`
	PreviousOutput       *string `json:"previous_output,omitempty"`
	TemplateBody         string  `json:"template_body"`
	PreviousTemplateBody *string `json:"previous_template_body,omitempty"`
}

type DeployPreviewResponse struct {
	Results              []TemplateRenderResult     `json:"results"`
	ValuesPayload        json.RawMessage            `json:"values_payload"`
	PreviousValues       *json.RawMessage           `json:"previous_values,omitempty"`
	ProjectVersionID     int64                      `json:"project_version_id"`
	GlobalValues         map[string]json.RawMessage `json:"global_values"`
	PreviousGlobalValues map[string]json.RawMessage `json:"previous_global_values,omitempty"`
	GroupVersions        map[string]int             `json:"global_values_group_versions"`
	HasErrors            bool                       `json:"has_errors"`
	// PreviousDeployment identifies the last successful deployment the preview
	// diffs against, or is nil when this project+env has never been deployed.
	PreviousDeployment *PreviousDeploymentInfo `json:"previous_deployment,omitempty"`
}

// PreviousDeploymentInfo is the baseline a deploy preview compares against,
// surfaced so the UI can label the diff ("vs deployment #N").
type PreviousDeploymentInfo struct {
	DeploymentID   int64     `json:"deployment_id"`
	ProjectVersion int       `json:"project_version"`
	CreatedAt      time.Time `json:"created_at"`
}

type DeployResponse struct {
	DeploymentID int64                  `json:"deployment_id"`
	Status       string                 `json:"status"`
	Results      []TemplateRenderResult `json:"results"`
}

type LatestDeploymentResponse struct {
	DeploymentID     int64          `json:"deployment_id"`
	ProjectVersionID int64          `json:"project_version_id"`
	GroupVersions    map[string]int `json:"global_values_group_versions"`
	CreatedAt        string         `json:"created_at"`
	CommitMessage    *string        `json:"commit_message"`
}
