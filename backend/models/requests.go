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

type AppendProjectConfigValuesVersionRequest struct {
	Payload       json.RawMessage `json:"payload"`
	CommitMessage *string         `json:"commit_message"`
}

// CreateGlobalValuesGroupRequest creates a new group whose v1 contains the
// provided values map (gv_name -> payload). At least one entry is required.
type CreateGlobalValuesGroupRequest struct {
	Name              string                     `json:"name"`
	Values            map[string]json.RawMessage `json:"values"`
	CommitMessage     *string                    `json:"commit_message"`
	ApprovalCondition *string                    `json:"approval_condition"`
}

// AppendGlobalValuesGroupVersionRequest appends a new group version
// snapshotting the provided values map. Unchanged entries are carried forward
// (their content rows are reused).
type AppendGlobalValuesGroupVersionRequest struct {
	Values        map[string]json.RawMessage `json:"values"`
	CommitMessage *string                    `json:"commit_message"`
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

// UpdateApprovalConditionRequest changes the PR-approval condition for a project
// or global values entry after creation.
type UpdateApprovalConditionRequest struct {
	ApprovalCondition string `json:"approval_condition"`
}

type AddProjectMemberRequest struct {
	UserID int64 `json:"user_id"`
}

// MemberPermissions is what a project admin can grant a member. Templates are
// project-wide (read/write/delete); value access is granted per environment so
// a member can be given, say, staging but not production. It is both the request
// body for setting permissions and the response shape for reading them; each
// enabled capability maps to an atom on the member's auto-managed per-member role.
type MemberPermissions struct {
	ReadTemplates   bool             `json:"read_templates"`
	WriteTemplates  bool             `json:"write_templates"`
	DeleteTemplates bool             `json:"delete_templates"`
	Environments    []EnvValueAccess `json:"environments"`
}

// EnvValueAccess is a member's value access to a single environment.
// read=true,write=false → "Read only"; write=true → "Read & write" (read is
// implied). An absent entry (or both false) means "No access".
type EnvValueAccess struct {
	Env   string `json:"env"`
	Read  bool   `json:"read"`
	Write bool   `json:"write"`
}

type AddEnvAdminRequest struct {
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

type SessionRequest struct {
	Token string `json:"token"`
}

type CreatePullRequestRequest struct {
	Title            string  `json:"title"`
	Description      *string `json:"description"`
	ObjectType       string  `json:"object_type"`
	GlobalValuesName *string `json:"global_values_name"`
	ProposedPayload  string  `json:"proposed_payload"`
}

type SubmitDraftRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
}

// DeployPreviewRequest / DeployRequest pin a single project version plus,
// for each referenced global-values group, the group version to use.
type DeployPreviewRequest struct {
	ProjectVersionID   int64          `json:"project_version_id"`
	GroupVersionIDsRaw map[string]int `json:"global_values_group_versions"` // group name -> group version ordinal
}

type DeployRequest struct {
	ProjectVersionID   int64          `json:"project_version_id"`
	GroupVersionIDsRaw map[string]int `json:"global_values_group_versions"`
	CommitMessage      *string        `json:"commit_message"`
}
