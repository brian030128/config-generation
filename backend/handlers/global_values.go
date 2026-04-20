package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/brian/config-generation/backend/models"
	"github.com/go-chi/chi/v5"
)

type GlobalValuesHandler struct {
	DB *sql.DB
}

// validateFlatJSON checks that the payload is a flat JSON object (scalars only, no nested objects/arrays).
func validateFlatJSON(payload json.RawMessage) error {
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return fmt.Errorf("payload must be a JSON object")
	}
	for key, val := range m {
		switch val.(type) {
		case string, float64, bool, nil:
			// valid scalar types
		default:
			return fmt.Errorf("key %q has a non-scalar value; only strings, numbers, booleans, and nulls are allowed", key)
		}
	}
	return nil
}

func (h *GlobalValuesHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	var req models.CreateGlobalValuesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
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

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer tx.Rollback()

	// 1. Insert global values entry.
	var gv models.GlobalValues
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO global_values (name, version_id, payload, commit_message, approval_condition, created_by)
		VALUES ($1, 1, $2, $3, COALESCE($4, '1 x gv_group_admin'), $5)
		RETURNING id, name, version_id, payload, commit_message, approval_condition, created_by, created_at
	`, req.Name, req.Payload, req.CommitMessage, req.ApprovalCondition, user.UserID).Scan(
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

	// 2. Auto-create gv_group_admin role.
	roleName := "gv_group_admin:" + gv.Name
	var roleID int64
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO roles (name, global_values_name, is_auto_created) VALUES ($1, $2, true)
		RETURNING id
	`, roleName, gv.Name).Scan(&roleID)
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
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, gv)
}

func (h *GlobalValuesHandler) AppendVersion(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	name := chi.URLParam(r, "name")

	var req models.AppendGlobalValuesVersionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
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
		writeError(w, http.StatusInternalServerError, "database error", "internal")
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
		writeError(w, http.StatusInternalServerError, "database error", "internal")
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
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, gv)
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
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	writeJSON(w, http.StatusOK, gv)
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
		writeError(w, http.StatusInternalServerError, "database error", "internal")
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
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer rows.Close()

	var entries []models.GlobalValues
	for rows.Next() {
		var gv models.GlobalValues
		if err := rows.Scan(&gv.ID, &gv.Name, &gv.VersionID, &gv.Payload, &gv.CommitMessage, &gv.ApprovalCondition, &gv.CreatedBy, &gv.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		entries = append(entries, gv)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
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
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer rows.Close()

	var versions []models.GlobalValues
	for rows.Next() {
		var gv models.GlobalValues
		if err := rows.Scan(&gv.ID, &gv.Name, &gv.VersionID, &gv.Payload, &gv.CommitMessage, &gv.ApprovalCondition, &gv.CreatedBy, &gv.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		versions = append(versions, gv)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	if versions == nil {
		versions = []models.GlobalValues{}
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.GlobalValues]{Items: versions, Count: len(versions)})
}
