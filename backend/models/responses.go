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

type TemplateVariable struct {
	Name    string  `json:"name"`
	Default *string `json:"default,omitempty"`
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
	Results              []TemplateRenderResult         `json:"results"`
	ValuesPayload        json.RawMessage                `json:"values_payload"`
	PreviousValues       *json.RawMessage               `json:"previous_values,omitempty"`
	ValuesVersionID      int                            `json:"values_version_id"`
	GlobalValues         map[string]json.RawMessage     `json:"global_values"`
	PreviousGlobalValues map[string]json.RawMessage     `json:"previous_global_values,omitempty"`
	GlobalValuesVersions map[string]int                 `json:"global_values_versions"`
	HasErrors            bool                           `json:"has_errors"`
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
