package handlers

import (
	"context"
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

// validateFlatJSON checks that a single gv payload is a flat object whose
// values are scalars or lists of strings. Nested objects are rejected.
func validateFlatJSON(payload json.RawMessage) error {
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return fmt.Errorf("payload must be a JSON object")
	}
	for key, val := range m {
		switch v := val.(type) {
		case string, float64, bool, nil:
			// scalar
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

func validateValuesMap(values map[string]json.RawMessage) error {
	if len(values) == 0 {
		return fmt.Errorf("values map must contain at least one entry")
	}
	for name, payload := range values {
		if name == "" {
			return fmt.Errorf("value name must be non-empty")
		}
		if err := validateFlatJSON(payload); err != nil {
			return fmt.Errorf("value %q: %v", name, err)
		}
	}
	return nil
}

// loadGroupVersionValues materializes the (gv_name -> payload) map for a group
// version by joining the entries to their underlying content rows.
func loadGroupVersionValues(ctx context.Context, db sqlQuerier, groupVersionID int64) (map[string]json.RawMessage, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT gv.name, gv.payload
		FROM global_values_group_version_entries e
		JOIN global_values gv ON gv.id = e.gv_row_id
		WHERE e.group_version_id = $1
		ORDER BY gv.name
	`, groupVersionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var name string
		var payload json.RawMessage
		if err := rows.Scan(&name, &payload); err != nil {
			return nil, err
		}
		out[name] = payload
	}
	return out, rows.Err()
}

// sqlQuerier is implemented by *sql.DB and *sql.Tx.
type sqlQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Create creates a new global-values group with its v1 snapshot.
func (h *GlobalValuesHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	var req models.CreateGlobalValuesGroupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required", "validation")
		return
	}
	if err := validateValuesMap(req.Values); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "validation")
		return
	}

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

	// 1. Group.
	var group models.GlobalValuesGroup
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO global_values_groups (name, approval_condition, created_by)
		VALUES ($1, $2, $3)
		RETURNING id, name, approval_condition, created_by, created_at
	`, req.Name, cond, user.UserID).Scan(
		&group.ID, &group.Name, &group.ApprovalCondition, &group.CreatedBy, &group.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "global values group already exists", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create group", "internal")
		return
	}

	// 2. v1 group version.
	var groupVersionID int64
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO global_values_group_versions (group_id, version_id, commit_message, created_by)
		VALUES ($1, 1, $2, $3)
		RETURNING id
	`, group.ID, req.CommitMessage, user.UserID).Scan(&groupVersionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create group version", "internal")
		return
	}

	// 3. Content rows + entries for each value in the v1 payload map.
	for name, payload := range req.Values {
		var rowID int64
		err = tx.QueryRowContext(r.Context(), `
			INSERT INTO global_values (group_id, name, payload, commit_message, created_by)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, group.ID, name, payload, req.CommitMessage, user.UserID).Scan(&rowID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create value row", "internal")
			return
		}
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO global_values_group_version_entries (group_version_id, gv_name, gv_row_id)
			VALUES ($1, $2, $3)
		`, groupVersionID, name, rowID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to link value to version", "internal")
			return
		}
	}

	// 4. Auto-created admin role + permissions.
	var roleID int64
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO roles (name, is_auto_created) VALUES ($1, true) RETURNING id
	`, adminRoleName).Scan(&roleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create admin role", "internal")
		return
	}
	adminPerms := []struct{ action, resource, keyName string }{
		{"write", "global_values", group.Name},
		{"delete", "global_values", group.Name},
		{"grant", "global_values", group.Name},
	}
	for _, p := range adminPerms {
		if _, err = tx.ExecContext(r.Context(), `
			INSERT INTO role_permissions (role_id, action, resource, key_name)
			VALUES ($1, $2, $3, $4)
		`, roleID, p.action, p.resource, p.keyName); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create role permissions", "internal")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO user_roles (user_id, role_id, granted_by) VALUES ($1, $2, $3)
	`, user.UserID, roleID, user.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to assign admin role", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, msgFailedToCommit, "internal")
		return
	}

	values, _ := loadGroupVersionValues(r.Context(), h.DB, groupVersionID)
	writeJSON(w, http.StatusCreated, models.GlobalValuesGroupDetailResponse{
		Group: group,
		LatestVersion: models.GlobalValuesGroupVersion{
			ID: groupVersionID, GroupID: group.ID, GroupName: group.Name,
			VersionID: 1, CommitMessage: req.CommitMessage, CreatedBy: user.UserID,
			CreatedAt: group.CreatedAt, Values: values,
		},
	})
}

// AppendVersion appends a new group version snapshotting the supplied payload
// map. Unchanged values reuse their previous content row.
func (h *GlobalValuesHandler) AppendVersion(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	groupName := chi.URLParam(r, "name")

	var req models.AppendGlobalValuesGroupVersionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if err := validateValuesMap(req.Values); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "validation")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tx.Rollback()

	var groupID int64
	err = tx.QueryRowContext(r.Context(), `SELECT id FROM global_values_groups WHERE name = $1 FOR UPDATE`, groupName).Scan(&groupID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "global values group not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	var prevGroupVersionID int64
	var nextOrd int
	err = tx.QueryRowContext(r.Context(), `
		SELECT id, version_id FROM global_values_group_versions
		WHERE group_id = $1 ORDER BY version_id DESC LIMIT 1
	`, groupID).Scan(&prevGroupVersionID, &nextOrd)
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	nextOrd++

	prevEntries, err := loadGroupVersionRowMap(r.Context(), tx, prevGroupVersionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	prevValues, err := loadGroupVersionValues(r.Context(), tx, prevGroupVersionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	var newVersionID int64
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO global_values_group_versions (group_id, version_id, parent_version_id, commit_message, created_by)
		VALUES ($1, $2, $3, $4, $5) RETURNING id
	`, groupID, nextOrd, prevGroupVersionID, req.CommitMessage, user.UserID).Scan(&newVersionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to append version", "internal")
		return
	}

	for name, payload := range req.Values {
		rowID, unchanged := prevEntries[name]
		if !unchanged || !rawJSONEqual(prevValues[name], payload) {
			err = tx.QueryRowContext(r.Context(), `
				INSERT INTO global_values (group_id, name, payload, commit_message, created_by)
				VALUES ($1, $2, $3, $4, $5) RETURNING id
			`, groupID, name, payload, req.CommitMessage, user.UserID).Scan(&rowID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to insert value content row", "internal")
				return
			}
		}
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO global_values_group_version_entries (group_version_id, gv_name, gv_row_id)
			VALUES ($1, $2, $3)
		`, newVersionID, name, rowID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to link value to version", "internal")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, msgFailedToCommit, "internal")
		return
	}

	values, _ := loadGroupVersionValues(r.Context(), h.DB, newVersionID)
	writeJSON(w, http.StatusCreated, models.GlobalValuesGroupVersion{
		ID: newVersionID, GroupID: groupID, GroupName: groupName,
		VersionID: nextOrd, CommitMessage: req.CommitMessage, CreatedBy: user.UserID,
		Values: values,
	})
}

// loadGroupVersionRowMap returns (gv_name -> content row id) for a group version.
func loadGroupVersionRowMap(ctx context.Context, db sqlQuerier, groupVersionID int64) (map[string]int64, error) {
	out := map[string]int64{}
	if groupVersionID == 0 {
		return out, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT gv_name, gv_row_id
		FROM global_values_group_version_entries
		WHERE group_version_id = $1
	`, groupVersionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var rowID int64
		if err := rows.Scan(&name, &rowID); err != nil {
			return nil, err
		}
		out[name] = rowID
	}
	return out, rows.Err()
}

// rawJSONEqual reports whether two raw JSON payloads are semantically equal
// (re-marshalled normalisation, sufficient for flat objects).
func rawJSONEqual(a, b json.RawMessage) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	var av, bv any
	if json.Unmarshal(a, &av) != nil || json.Unmarshal(b, &bv) != nil {
		return false
	}
	ab, _ := json.Marshal(av)
	bb, _ := json.Marshal(bv)
	return string(ab) == string(bb)
}

// UpdateApprovalCondition replaces the group's approval condition.
func (h *GlobalValuesHandler) UpdateApprovalCondition(w http.ResponseWriter, r *http.Request) {
	groupName := chi.URLParam(r, "name")

	var exists bool
	err := h.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM global_values_groups WHERE name = $1)`, groupName).Scan(&exists)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "global values group not found", "not_found")
		return
	}

	var req models.UpdateApprovalConditionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if err := validateApprovalCondition(r.Context(), h.DB, []string{groupName + "_gv_group_admin"}, req.ApprovalCondition); err != nil {
		writeApprovalConditionError(w, err)
		return
	}

	if _, err = h.DB.ExecContext(r.Context(), `
		UPDATE global_values_groups SET approval_condition = $1 WHERE name = $2
	`, strings.TrimSpace(req.ApprovalCondition), groupName); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update approval condition", "internal")
		return
	}

	var group models.GlobalValuesGroup
	if err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, name, approval_condition, created_by, created_at FROM global_values_groups WHERE name = $1
	`, groupName).Scan(&group.ID, &group.Name, &group.ApprovalCondition, &group.CreatedBy, &group.CreatedAt); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, group)
}

// GetLatest returns the group plus its latest materialized version.
func (h *GlobalValuesHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	groupName := chi.URLParam(r, "name")

	var group models.GlobalValuesGroup
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT id, name, approval_condition, created_by, created_at
		FROM global_values_groups WHERE name = $1
	`, groupName).Scan(&group.ID, &group.Name, &group.ApprovalCondition, &group.CreatedBy, &group.CreatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "global values group not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	var gvv models.GlobalValuesGroupVersion
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, group_id, version_id, parent_version_id, commit_message, created_by, created_at
		FROM global_values_group_versions WHERE group_id = $1 ORDER BY version_id DESC LIMIT 1
	`, group.ID).Scan(&gvv.ID, &gvv.GroupID, &gvv.VersionID, &gvv.ParentVersionID, &gvv.CommitMessage, &gvv.CreatedBy, &gvv.CreatedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	gvv.GroupName = group.Name
	gvv.Values, _ = loadGroupVersionValues(r.Context(), h.DB, gvv.ID)

	canManage, err := middleware.CheckPermission(r.Context(), h.DB, currentUser(r).UserID, models.PermissionRequirement{
		Action: models.ActionGrant, Resource: models.ResourceGlobalValues, KeyName: groupName,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "permission check failed", "internal")
		return
	}

	writeJSON(w, http.StatusOK, models.GlobalValuesGroupDetailResponse{
		Group: group, LatestVersion: gvv, ViewerCanManage: canManage,
	})
}

// GetVersion returns one materialized group version.
func (h *GlobalValuesHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	groupName := chi.URLParam(r, "name")
	versionID, err := strconv.Atoi(chi.URLParam(r, "versionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version ID", "bad_request")
		return
	}

	var gvv models.GlobalValuesGroupVersion
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT ggv.id, ggv.group_id, g.name, ggv.version_id, ggv.parent_version_id, ggv.commit_message, ggv.created_by, ggv.created_at
		FROM global_values_group_versions ggv
		JOIN global_values_groups g ON g.id = ggv.group_id
		WHERE g.name = $1 AND ggv.version_id = $2
	`, groupName, versionID).Scan(
		&gvv.ID, &gvv.GroupID, &gvv.GroupName, &gvv.VersionID,
		&gvv.ParentVersionID, &gvv.CommitMessage, &gvv.CreatedBy, &gvv.CreatedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "global values group version not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	gvv.Values, _ = loadGroupVersionValues(r.Context(), h.DB, gvv.ID)
	writeJSON(w, http.StatusOK, gvv)
}

// List returns all groups with their latest version metadata (no values map).
func (h *GlobalValuesHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, name, approval_condition, created_by, created_at
		FROM global_values_groups ORDER BY name
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer rows.Close()

	groups := []models.GlobalValuesGroup{}
	for rows.Next() {
		var g models.GlobalValuesGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.ApprovalCondition, &g.CreatedBy, &g.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.GlobalValuesGroup]{Items: groups, Count: len(groups)})
}

// ListVersions returns all versions of a group (metadata only, no values map).
func (h *GlobalValuesHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	groupName := chi.URLParam(r, "name")

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT ggv.id, ggv.group_id, g.name, ggv.version_id, ggv.parent_version_id, ggv.commit_message, ggv.created_by, ggv.created_at
		FROM global_values_group_versions ggv
		JOIN global_values_groups g ON g.id = ggv.group_id
		WHERE g.name = $1
		ORDER BY ggv.version_id DESC
	`, groupName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer rows.Close()

	versions := []models.GlobalValuesGroupVersion{}
	for rows.Next() {
		var v models.GlobalValuesGroupVersion
		if err := rows.Scan(&v.ID, &v.GroupID, &v.GroupName, &v.VersionID, &v.ParentVersionID, &v.CommitMessage, &v.CreatedBy, &v.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.GlobalValuesGroupVersion]{Items: versions, Count: len(versions)})
}
