package models

import (
	"encoding/json"
	"time"
)

type User struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	DisplayName *string   `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type Environment struct {
	ID          int64     `json:"id"`
	ProjectID   int64     `json:"project_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedBy   int64     `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type Project struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	Description       *string   `json:"description"`
	ApprovalCondition string    `json:"approval_condition"`
	CreatedBy         int64     `json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ProjectMember is a user's membership in a project. Membership grants
// read:project(name) and nothing more.
type ProjectMember struct {
	UserID      int64     `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName *string   `json:"display_name"`
	AddedBy     int64     `json:"added_by"`
	AddedAt     time.Time `json:"added_at"`
}

// EnvAdmin is a user's env-admin grant on a single environment. Mirrors
// ProjectMember; the synthesized atoms are derived in the permission loader.
type EnvAdmin struct {
	UserID      int64     `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName *string   `json:"display_name"`
	GrantedBy   int64     `json:"granted_by"`
	GrantedAt   time.Time `json:"granted_at"`
}

// ProjectConfigTemplate is an immutable content row holding one template body.
// Authoritative versioning is per project (see ProjectVersion); VersionID is a
// monotonic per-(project, template_name) counter for traceability and as a
// convenience for clients that want a "this is the Nth time this template
// changed" identifier without consulting the snapshot tables.
type ProjectConfigTemplate struct {
	ID            int64     `json:"id"`
	ProjectID     int64     `json:"project_id"`
	TemplateName  string    `json:"template_name"`
	VersionID     int       `json:"version_id"`
	Body          string    `json:"body"`
	CommitMessage *string   `json:"commit_message"`
	CreatedBy     int64     `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

// ProjectConfigValues is an immutable content row for one (project, env)
// values payload. VersionID is a per-(project, env) monotonic counter, same
// semantics as ProjectConfigTemplate.VersionID.
type ProjectConfigValues struct {
	ID            int64           `json:"id"`
	ProjectID     int64           `json:"project_id"`
	EnvironmentID int64           `json:"environment_id"`
	VersionID     int             `json:"version_id"`
	Payload       json.RawMessage `json:"payload"`
	CommitMessage *string         `json:"commit_message"`
	CreatedBy     int64           `json:"created_by"`
	CreatedAt     time.Time       `json:"created_at"`
}

// GlobalValues is an immutable content row for one global value inside a group.
// Versioning lives on the owning group (see GlobalValuesGroupVersion).
type GlobalValues struct {
	ID            int64           `json:"id"`
	GroupID       int64           `json:"group_id"`
	Name          string          `json:"name"`
	Payload       json.RawMessage `json:"payload"`
	CommitMessage *string         `json:"commit_message"`
	CreatedBy     int64           `json:"created_by"`
	CreatedAt     time.Time       `json:"created_at"`
}

// GlobalValuesGroup is a named bundle of global values with its own version
// sequence; each version snapshots every value in the group.
type GlobalValuesGroup struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	ApprovalCondition string    `json:"approval_condition"`
	CreatedBy         int64     `json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
}

type GlobalValuesGroupVersion struct {
	ID              int64     `json:"id"`
	GroupID         int64     `json:"group_id"`
	GroupName       string    `json:"group_name"`
	VersionID       int       `json:"version_id"`
	ParentVersionID *int64    `json:"parent_version_id"`
	CommitMessage   *string   `json:"commit_message"`
	CreatedBy       int64     `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
	// Values is the materialized payload map for this group version: gv_name -> payload.
	// Populated by handlers that read the version's full contents.
	Values map[string]json.RawMessage `json:"values,omitempty"`
}

// ProjectVersion is a per-project snapshot covering all templates and all
// environment values for the project at that point in time. Each commit
// produces one new project version via copy-on-write.
type ProjectVersion struct {
	ID              int64     `json:"id"`
	ProjectID       int64     `json:"project_id"`
	VersionID       int       `json:"version_id"`
	ParentVersionID *int64    `json:"parent_version_id"`
	CommitMessage   *string   `json:"commit_message"`
	IsAnchor        bool      `json:"is_anchor"`
	CreatedBy       int64     `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
}

// ProjectVersionManifest lists what content rows a project version snapshots.
type ProjectVersionManifest struct {
	ProjectVersion
	Templates []ProjectConfigTemplate `json:"templates"`
	Values    []ProjectConfigValues   `json:"values"`
}

// Role is a global, named bundle of permission atoms. Its scope comes only from
// the atoms' keys (key_project/key_env/key_name); the role itself is not bound to
// any project or global values entry.
type Role struct {
	ID            int64            `json:"id"`
	Name          string           `json:"name"`
	IsAutoCreated bool             `json:"is_auto_created"`
	CreatedAt     time.Time        `json:"created_at"`
	Permissions   []RolePermission `json:"permissions,omitempty"`
	Members       []UserRole       `json:"members,omitempty"`
}

type RolePermission struct {
	ID         int64   `json:"id"`
	RoleID     int64   `json:"role_id"`
	Action     string  `json:"action"`
	Resource   string  `json:"resource"`
	KeyProject *string `json:"key_project"`
	KeyEnv     *string `json:"key_env"`
	KeyName    *string `json:"key_name"`
}

type UserRole struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	RoleID    int64     `json:"role_id"`
	GrantedBy int64     `json:"granted_by"`
	GrantedAt time.Time `json:"granted_at"`
	// Username and DisplayName are populated only by the role-list endpoints
	// (joined from users) so the UI can render assigned members by name.
	Username    string  `json:"username,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
}

type PullRequest struct {
	ID                   int64        `json:"id"`
	ProjectID            *int64       `json:"project_id"`
	GlobalValuesName     *string      `json:"global_values_name"`
	BaseProjectVersionID *int64       `json:"base_project_version_id"`
	BaseGroupVersionID   *int64       `json:"base_group_version_id"`
	AuthorID             int64        `json:"author_id"`
	Title                string       `json:"title"`
	Description          *string      `json:"description"`
	Status               string       `json:"status"`
	IsConflicted         bool         `json:"is_conflicted"`
	ApprovalCondition    string       `json:"approval_condition"`
	CreatedAt            time.Time    `json:"created_at"`
	UpdatedAt            time.Time    `json:"updated_at"`
	MergedAt             *time.Time   `json:"merged_at"`
	ClosedAt             *time.Time   `json:"closed_at"`
	Changes              []PRChange   `json:"changes,omitempty"`
	Approvals            []PRApproval `json:"approvals,omitempty"`
}

type PRApproval struct {
	ID          int64      `json:"id"`
	PRID        int64      `json:"pr_id"`
	UserID      int64      `json:"user_id"`
	ApprovedAt  time.Time  `json:"approved_at"`
	WithdrawnAt *time.Time `json:"withdrawn_at"`
}

type PRChange struct {
	ID               int64     `json:"id"`
	PRID             int64     `json:"pr_id"`
	ObjectType       string    `json:"object_type"`
	Operation        string    `json:"operation"`
	ProjectID        *int64    `json:"project_id"`
	TemplateName     *string   `json:"template_name"`
	EnvironmentName  *string   `json:"environment_name"`
	GlobalValuesName *string   `json:"global_values_name"`
	ProposedPayload  string    `json:"proposed_payload"`
	CreatedAt        time.Time `json:"created_at"`
}

type Deployment struct {
	ID               int64     `json:"id"`
	ProjectID        int64     `json:"project_id"`
	EnvironmentID    int64     `json:"environment_id"`
	ProjectVersionID int64     `json:"project_version_id"`
	Status           string    `json:"status"`
	RolledBackFrom   *int64    `json:"rolled_back_from"`
	CommitMessage    *string   `json:"commit_message"`
	CreatedBy        int64     `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
}

type DeploymentEntry struct {
	ID             int64  `json:"id"`
	DeploymentID   int64  `json:"deployment_id"`
	TemplateName   string `json:"template_name"`
	RenderedOutput string `json:"rendered_output"`
}

// DeploymentGroupRef pins one global-values-group version per deployment.
type DeploymentGroupRef struct {
	ID             int64  `json:"id"`
	DeploymentID   int64  `json:"deployment_id"`
	GroupID        int64  `json:"group_id"`
	GroupName      string `json:"group_name"`
	GroupVersionID int64  `json:"group_version_id"`
	VersionID      int    `json:"version_id"`
}
