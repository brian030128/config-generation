package models

import "encoding/json"

type ListResponse[T any] struct {
	Items []T `json:"items"`
	Count int `json:"count"`
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

type AuthConfigResponse struct {
	OIDCEnabled          bool   `json:"oidc_enabled"`
	OIDCProviderName     string `json:"oidc_provider_name"`
	PasswordLoginEnabled bool   `json:"password_login_enabled"`
	RegistrationEnabled  bool   `json:"registration_enabled"`
}

type TemplateVariable struct {
	Name    string  `json:"name"`
	Default *string `json:"default,omitempty"`
}

// Workspace overlay views: the published base merged with the caller's own
// staged changes. `staged` marks an item touched by the active workspace;
// `operation` is the staged operation (create|update|delete) when staged.

type WorkspaceTemplateItem struct {
	TemplateName string `json:"template_name"`
	Body         string `json:"body"`
	VersionID    int    `json:"version_id"` // live latest version; 0 if new in the workspace
	Staged       bool   `json:"staged"`
	Operation    string `json:"operation,omitempty"`
}

type WorkspaceEnvironmentItem struct {
	Name      string `json:"name"`
	Staged    bool   `json:"staged"`
	Operation string `json:"operation,omitempty"`
}

type WorkspaceValuesResponse struct {
	EnvironmentName string          `json:"environment_name"`
	Payload         json.RawMessage `json:"payload"`
	VersionID       int             `json:"version_id"` // live latest version; 0 if staged-new
	Staged          bool            `json:"staged"`
	Operation       string          `json:"operation,omitempty"`
}

type TemplateVariablesResponse struct {
	Variables []TemplateVariable `json:"variables"`
}

type TemplateRenderResult struct {
	TemplateName         string  `json:"template_name"`
	RenderedOutput       *string `json:"rendered_output,omitempty"`
	Error                *string `json:"error,omitempty"`
	ErrorKind            *string `json:"error_kind,omitempty"`
	PreviousOutput       *string `json:"previous_output,omitempty"`
	TemplateBody         string  `json:"template_body"`
	PreviousTemplateBody *string `json:"previous_template_body,omitempty"`
	TemplateVersionID    int     `json:"template_version_id"`
}

type DeployPreviewResponse struct {
	Results              []TemplateRenderResult     `json:"results"`
	ValuesPayload        json.RawMessage            `json:"values_payload"`
	PreviousValues       *json.RawMessage           `json:"previous_values,omitempty"`
	ValuesVersionID      int                        `json:"values_version_id"`
	GlobalValues         map[string]json.RawMessage `json:"global_values"`
	PreviousGlobalValues map[string]json.RawMessage `json:"previous_global_values,omitempty"`
	GlobalValuesVersions map[string]int             `json:"global_values_versions"`
	HasErrors            bool                       `json:"has_errors"`
}

type DeployResponse struct {
	DeploymentID int64                  `json:"deployment_id"`
	Status       string                 `json:"status"`
	Results      []TemplateRenderResult `json:"results"`
}

type LatestDeploymentResponse struct {
	DeploymentID         int64          `json:"deployment_id"`
	TemplateVersions     map[string]int `json:"template_versions"`
	ValuesVersionID      int            `json:"values_version_id"`
	GlobalValuesVersions map[string]int `json:"global_values_versions"`
	CreatedAt            string         `json:"created_at"`
	CommitMessage        *string        `json:"commit_message"`
}
