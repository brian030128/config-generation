package models

import "encoding/json"

type CreateProjectRequest struct {
	Name              string  `json:"name"`
	Description       *string `json:"description"`
	ApprovalCondition *string `json:"approval_condition"`
}

type CreateEnvironmentRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type CreateTemplateRequest struct {
	TemplateName  string  `json:"template_name"`
	Body          string  `json:"body"`
	CommitMessage *string `json:"commit_message"`
}

type AppendTemplateVersionRequest struct {
	Body          string  `json:"body"`
	CommitMessage *string `json:"commit_message"`
}

type CreateProjectConfigValuesRequest struct {
	EnvironmentID int64           `json:"environment_id"`
	Payload       json.RawMessage `json:"payload"`
	CommitMessage *string         `json:"commit_message"`
}

type AppendProjectConfigValuesVersionRequest struct {
	Payload       json.RawMessage `json:"payload"`
	CommitMessage *string         `json:"commit_message"`
}

type CreateGlobalValuesRequest struct {
	Name              string          `json:"name"`
	Payload           json.RawMessage `json:"payload"`
	CommitMessage     *string         `json:"commit_message"`
	ApprovalCondition *string         `json:"approval_condition"`
}

type AppendGlobalValuesVersionRequest struct {
	Payload       json.RawMessage `json:"payload"`
	CommitMessage *string         `json:"commit_message"`
}

type CreateRoleRequest struct {
	Name        string                `json:"name"`
	ProjectID   *int64                `json:"project_id"`
	Permissions []PermissionAtomInput `json:"permissions"`
}

type PermissionAtomInput struct {
	Action     string  `json:"action"`
	Resource   string  `json:"resource"`
	KeyProject *string `json:"key_project"`
	KeyEnv     *string `json:"key_env"`
	KeyName    *string `json:"key_name"`
}

type EditRolePermissionsRequest struct {
	Permissions []PermissionAtomInput `json:"permissions"`
}

type AssignUserRoleRequest struct {
	UserID int64 `json:"user_id"`
}

type RegisterRequest struct {
	Username    string  `json:"username"`
	Password    string  `json:"password"`
	DisplayName *string `json:"display_name"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreatePullRequestRequest struct {
	Title            string  `json:"title"`
	Description      *string `json:"description"`
	ObjectType       string  `json:"object_type"`
	GlobalValuesName *string `json:"global_values_name"`
	ProposedPayload  string  `json:"proposed_payload"`
}

type StageChangeRequest struct {
	ObjectType      string `json:"object_type"`
	TemplateName    string `json:"template_name,omitempty"`
	EnvironmentName string `json:"environment_name,omitempty"`
	ProposedPayload string `json:"proposed_payload"`
}

type SubmitDraftRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
}

type DeployPreviewRequest struct {
	TemplateVersions     map[string]int `json:"template_versions"`
	ValuesVersionID      int            `json:"values_version_id"`
	GlobalValuesVersions map[string]int `json:"global_values_versions"`
}

type DeployRequest struct {
	TemplateVersions     map[string]int `json:"template_versions"`
	ValuesVersionID      int            `json:"values_version_id"`
	GlobalValuesVersions map[string]int `json:"global_values_versions"`
	CommitMessage        *string        `json:"commit_message"`
}
