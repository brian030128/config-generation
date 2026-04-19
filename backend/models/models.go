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

type GlobalValues struct {
	ID                int64           `json:"id"`
	Name              string          `json:"name"`
	VersionID         int             `json:"version_id"`
	Payload           json.RawMessage `json:"payload"`
	CommitMessage     *string         `json:"commit_message"`
	ApprovalCondition string          `json:"approval_condition"`
	CreatedBy         int64           `json:"created_by"`
	CreatedAt         time.Time       `json:"created_at"`
}

type Role struct {
	ID               int64            `json:"id"`
	Name             string           `json:"name"`
	ProjectID        *int64           `json:"project_id"`
	GlobalValuesName *string          `json:"global_values_name"`
	IsAutoCreated    bool             `json:"is_auto_created"`
	CreatedAt        time.Time        `json:"created_at"`
	Permissions      []RolePermission `json:"permissions,omitempty"`
	Members          []UserRole       `json:"members,omitempty"`
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
}

type PullRequest struct {
	ID                int64        `json:"id"`
	ProjectID         *int64       `json:"project_id"`
	GlobalValuesName  *string      `json:"global_values_name"`
	AuthorID          int64        `json:"author_id"`
	Title             string       `json:"title"`
	Description       *string      `json:"description"`
	Status            string       `json:"status"`
	IsConflicted      bool         `json:"is_conflicted"`
	ApprovalCondition string       `json:"approval_condition"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
	MergedAt          *time.Time   `json:"merged_at"`
	ClosedAt          *time.Time   `json:"closed_at"`
	Changes           []PRChange   `json:"changes,omitempty"`
	Approvals         []PRApproval `json:"approvals,omitempty"`
}

type PRApproval struct {
	ID          int64      `json:"id"`
	PRID        int64      `json:"pr_id"`
	UserID      int64      `json:"user_id"`
	ApprovedAt  time.Time  `json:"approved_at"`
	WithdrawnAt *time.Time `json:"withdrawn_at"`
}

type PRChange struct {
	ID               int64           `json:"id"`
	PRID             int64           `json:"pr_id"`
	ObjectType       string          `json:"object_type"`
	ProjectID        *int64          `json:"project_id"`
	TemplateName     *string         `json:"template_name"`
	EnvironmentName  *string         `json:"environment_name"`
	GlobalValuesName *string         `json:"global_values_name"`
	BaseVersionID    int             `json:"base_version_id"`
	ProposedPayload  string          `json:"proposed_payload"`
	CreatedAt        time.Time       `json:"created_at"`
}
