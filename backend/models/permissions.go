package models

// Actions
const (
	ActionRead   = "read"
	ActionWrite  = "write"
	ActionCreate = "create"
	ActionDelete = "delete"
	ActionGrant  = "grant"
)

// Resources
const (
	ResourceProjectTemplates = "project_templates"
	ResourceProjectValues    = "project_values"
	ResourceGlobalValues     = "global_values"
	ResourceProject          = "project"
	ResourceEnvValues        = "env_values"
)

// PermissionRequirement describes what permission a route needs.
// Empty string keys mean "not applicable for this resource type."
type PermissionRequirement struct {
	Action     string
	Resource   string
	KeyProject string
	KeyEnv     string
	KeyName    string
}
