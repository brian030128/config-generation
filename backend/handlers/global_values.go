package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/brian/config-generation/backend/middleware"
	"github.com/brian/config-generation/backend/models"
	"github.com/go-chi/chi/v5"
)

type GlobalValuesHandler struct {
	DB *sql.DB
}

// validateFlatJSON checks that the payload is a flat JSON object: each value must be a
// scalar (string, number, boolean, null) or a list of strings. Nested objects and lists
// containing anything other than strings are rejected.
func validateFlatJSON(payload json.RawMessage) error {
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return fmt.Errorf("payload must be a JSON object")
	}
	for key, val := range m {
		switch v := val.(type) {
		case string, float64, bool, nil:
			// valid scalar types
		case []any:
			for i, el := range v {
				if _, ok := el.(string); !ok {
					return fmt.Errorf("key %q list item %d must be a string", key, i)
				}
			}
		default:
			return fmt.Errorf("key %q must be a string, number, boolean, null, or a list of strings", key)
		}
	}
	return nil
}

func (h *GlobalValuesHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	var req models.CreateGlobalValuesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required", "validation")
		return
	}
	if len(req.Payload) == 0 {
		writeError(w, http.StatusBadRequest, "payload is required", "validation")
		return
	}
	if err := validateFlatJSON(req.Payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "validation")
		return
	}

	// Roles are global; an entry's auto-created admin role is named
	// "<name>_gv_group_admin". The condition defaults to requiring it and, at
	// creation, may only reference it.
	adminRoleName := req.Name + "_gv_group_admin"
	cond := "1 x " + adminRoleName
	if req.ApprovalCondition != nil {
		cond = *req.ApprovalCondition
	}
	if err := validateApprovalCondition(r.Context(), h.DB, []string{adminRoleName}, cond); err != nil {
		writeApprovalConditionError(w, err)
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tx.Rollback()

	// 1. Insert global values entry.
	var gv models.GlobalValues
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO global_values (name, version_id, payload, commit_message, approval_condition, created_by)
		VALUES ($1, 1, $2, $3, $4, $5)
		RETURNING id, name, version_id, payload, commit_message, approval_condition, created_by, created_at
	`, req.Name, req.Payload, req.CommitMessage, cond, user.UserID).Scan(
		&gv.ID, &gv.Name, &gv.VersionID, &gv.Payload,
		&gv.CommitMessage, &gv.ApprovalCondition, &gv.CreatedBy, &gv.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "global values entry already exists", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create global values", "internal")
		return
	}

	// 2. Auto-create the entry's admin role (global; scope comes from its atoms).
	var roleID int64
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO roles (name, is_auto_created) VALUES ($1, true)
		RETURNING id
	`, adminRoleName).Scan(&roleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create admin role", "internal")
		return
	}

	// 3. Insert permission atoms for the admin role.
	adminPerms := []struct {
		action, resource, keyName string
	}{
		{"write", "global_values", gv.Name},
		{"delete", "global_values", gv.Name},
		{"grant", "global_values", gv.Name},
	}
	for _, p := range adminPerms {
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO role_permissions (role_id, action, resource, key_name)
			VALUES ($1, $2, $3, $4)
		`, roleID, p.action, p.resource, p.keyName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create role permissions", "internal")
			return
		}
	}

	// 4. Assign role to creator.
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO user_roles (user_id, role_id, granted_by) VALUES ($1, $2, $3)
	`, user.UserID, roleID, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to assign admin role", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, msgFailedToCommit, "internal")
		return
	}

	writeJSON(w, http.StatusCreated, gv)
}

func (h *GlobalValuesHandler) AppendVersion(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	name := chi.URLParam(r, "name")

	var req models.AppendGlobalValuesVersionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if len(req.Payload) == 0 {
		writeError(w, http.StatusBadRequest, "payload is required", "validation")
		return
	}
	if err := validateFlatJSON(req.Payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "validation")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tx.Rollback()

	var nextVersion int
	err = tx.QueryRowContext(r.Context(), `
		SELECT COALESCE(
			(SELECT version_id FROM global_values
			 WHERE name = $1
			 ORDER BY version_id DESC LIMIT 1
			 FOR UPDATE),
		0) + 1
	`, name).Scan(&nextVersion)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	if nextVersion == 1 {
		writeError(w, http.StatusNotFound, "global values entry not found, use create instead", "not_found")
		return
	}

	var gv models.GlobalValues
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO global_values (name, version_id, payload, commit_message, approval_condition, created_by)
		VALUES ($1, $2, $3, $4,
			(SELECT approval_condition FROM global_values WHERE name = $1 ORDER BY version_id LIMIT 1),
			$5)
		RETURNING id, name, version_id, payload, commit_message, approval_condition, created_by, created_at
	`, name, nextVersion, req.Payload, req.CommitMessage, user.UserID).Scan(
		&gv.ID, &gv.Name, &gv.VersionID, &gv.Payload,
		&gv.CommitMessage, &gv.ApprovalCondition, &gv.CreatedBy, &gv.CreatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to append version", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, msgFailedToCommit, "internal")
		return
	}

	writeJSON(w, http.StatusCreated, gv)
}

// UpdateApprovalCondition changes a global values entry's PR-approval condition.
// The condition must be parseable and may only reference roles that exist in the
// entry's scope (plus the built-in gv_group_admin). It is stored on every version
// row (the evaluator reads v1), so all rows for the name are updated.
func (h *GlobalValuesHandler) UpdateApprovalCondition(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var exists bool
	err := h.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM global_values WHERE name = $1)`, name).Scan(&exists)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "global values entry not found", "not_found")
		return
	}

	var req models.UpdateApprovalConditionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}

	if err := validateApprovalCondition(r.Context(), h.DB, []string{name + "_gv_group_admin"}, req.ApprovalCondition); err != nil {
		writeApprovalConditionError(w, err)
		return
	}

	if _, err = h.DB.ExecContext(r.Context(), `
		UPDATE global_values SET approval_condition = $1 WHERE name = $2
	`, strings.TrimSpace(req.ApprovalCondition), name); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update approval condition", "internal")
		return
	}

	var gv models.GlobalValues
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, name, version_id, payload, commit_message, approval_condition, created_by, created_at
		FROM global_values WHERE name = $1 ORDER BY version_id DESC LIMIT 1
	`, name).Scan(&gv.ID, &gv.Name, &gv.VersionID, &gv.Payload, &gv.CommitMessage, &gv.ApprovalCondition, &gv.CreatedBy, &gv.CreatedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, gv)
}

func (h *GlobalValuesHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var gv models.GlobalValues
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT id, name, version_id, payload, commit_message, approval_condition, created_by, created_at
		FROM global_values
		WHERE name = $1
		ORDER BY version_id DESC LIMIT 1
	`, name).Scan(
		&gv.ID, &gv.Name, &gv.VersionID, &gv.Payload,
		&gv.CommitMessage, &gv.ApprovalCondition, &gv.CreatedBy, &gv.CreatedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "global values entry not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	// ViewerCanManage gates the approval-policy editor in the UI: grant on the entry.
	canManage, err := middleware.CheckPermission(r.Context(), h.DB, currentUser(r).UserID, models.PermissionRequirement{
		Action:   models.ActionGrant,
		Resource: models.ResourceGlobalValues,
		KeyName:  name,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "permission check failed", "internal")
		return
	}

	writeJSON(w, http.StatusOK, models.GlobalValuesDetailResponse{GlobalValues: gv, ViewerCanManage: canManage})
}

func (h *GlobalValuesHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	versionID, err := strconv.Atoi(chi.URLParam(r, "versionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version ID", "bad_request")
		return
	}

	var gv models.GlobalValues
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, name, version_id, payload, commit_message, approval_condition, created_by, created_at
		FROM global_values
		WHERE name = $1 AND version_id = $2
	`, name, versionID).Scan(
		&gv.ID, &gv.Name, &gv.VersionID, &gv.Payload,
		&gv.CommitMessage, &gv.ApprovalCondition, &gv.CreatedBy, &gv.CreatedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "global values version not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, gv)
}

func (h *GlobalValuesHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT DISTINCT ON (name)
			id, name, version_id, payload, commit_message, approval_condition, created_by, created_at
		FROM global_values
		ORDER BY name, version_id DESC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer rows.Close()

	var entries []models.GlobalValues
	for rows.Next() {
		var gv models.GlobalValues
		if err := rows.Scan(&gv.ID, &gv.Name, &gv.VersionID, &gv.Payload, &gv.CommitMessage, &gv.ApprovalCondition, &gv.CreatedBy, &gv.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		entries = append(entries, gv)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	if entries == nil {
		entries = []models.GlobalValues{}
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.GlobalValues]{Items: entries, Count: len(entries)})
}

func (h *GlobalValuesHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, name, version_id, payload, commit_message, approval_condition, created_by, created_at
		FROM global_values
		WHERE name = $1
		ORDER BY version_id DESC
	`, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer rows.Close()

	var versions []models.GlobalValues
	for rows.Next() {
		var gv models.GlobalValues
		if err := rows.Scan(&gv.ID, &gv.Name, &gv.VersionID, &gv.Payload, &gv.CommitMessage, &gv.ApprovalCondition, &gv.CreatedBy, &gv.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		versions = append(versions, gv)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	if versions == nil {
		versions = []models.GlobalValues{}
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.GlobalValues]{Items: versions, Count: len(versions)})
}
