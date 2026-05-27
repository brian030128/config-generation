package handlers

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/brian/config-generation/backend/middleware"
	"github.com/brian/config-generation/backend/models"
)

type RoleHandler struct {
	DB *sql.DB
}

// requireSuperuser writes a 403 and returns false unless the caller is a
// superuser. Global role management (create/edit/delete/assign) is superuser-only.
func (h *RoleHandler) requireSuperuser(w http.ResponseWriter, r *http.Request) bool {
	su, err := middleware.IsSuperuser(r.Context(), h.DB, currentUser(r).UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "permission check failed", "internal")
		return false
	}
	if !su {
		writeError(w, http.StatusForbidden, "only superusers can manage roles", "forbidden")
		return false
	}
	return true
}

// loadRole fetches a role by its {roleID} URL parameter.
func (h *RoleHandler) loadRole(w http.ResponseWriter, r *http.Request) (*models.Role, bool) {
	roleID, err := urlParamInt64(r, "roleID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid role ID", "bad_request")
		return nil, false
	}
	var role models.Role
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, name, is_auto_created, created_at FROM roles WHERE id = $1
	`, roleID).Scan(&role.ID, &role.Name, &role.IsAutoCreated, &role.CreatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "role not found", "not_found")
		return nil, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return nil, false
	}
	return &role, true
}

// Create creates a new global role.
func (h *RoleHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.requireSuperuser(w, r) {
		return
	}

	var req models.CreateRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required", "validation")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer tx.Rollback()

	var role models.Role
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO roles (name, is_auto_created) VALUES ($1, false)
		RETURNING id, name, is_auto_created, created_at
	`, req.Name).Scan(&role.ID, &role.Name, &role.IsAutoCreated, &role.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "role name already exists", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create role", "internal")
		return
	}

	if err := insertRolePermissions(r.Context(), tx, role.ID, req.Permissions); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create role permissions", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, role)
}

// EditPermissions replaces all permissions for a custom role.
func (h *RoleHandler) EditPermissions(w http.ResponseWriter, r *http.Request) {
	if !h.requireSuperuser(w, r) {
		return
	}
	role, ok := h.loadRole(w, r)
	if !ok {
		return
	}
	if role.IsAutoCreated {
		writeError(w, http.StatusBadRequest, "cannot edit auto-created role permissions", "validation")
		return
	}

	var req models.EditRolePermissionsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(r.Context(), `DELETE FROM role_permissions WHERE role_id = $1`, role.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear permissions", "internal")
		return
	}
	if err := insertRolePermissions(r.Context(), tx, role.ID, req.Permissions); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create role permissions", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Delete deletes a custom role and revokes all member assignments.
func (h *RoleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.requireSuperuser(w, r) {
		return
	}
	role, ok := h.loadRole(w, r)
	if !ok {
		return
	}
	if role.IsAutoCreated {
		writeError(w, http.StatusBadRequest, "cannot delete auto-created roles", "validation")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer tx.Rollback()

	for _, q := range []string{
		`DELETE FROM user_roles WHERE role_id = $1`,
		`DELETE FROM role_permissions WHERE role_id = $1`,
		`DELETE FROM roles WHERE id = $1`,
	} {
		if _, err := tx.ExecContext(r.Context(), q, role.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete role", "internal")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AssignUser assigns a user to a role.
func (h *RoleHandler) AssignUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireSuperuser(w, r) {
		return
	}
	role, ok := h.loadRole(w, r)
	if !ok {
		return
	}

	var req models.AssignUserRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	if req.UserID == 0 {
		writeError(w, http.StatusBadRequest, "user_id is required", "validation")
		return
	}

	user := currentUser(r)
	var ur models.UserRole
	err := h.DB.QueryRowContext(r.Context(), `
		INSERT INTO user_roles (user_id, role_id, granted_by) VALUES ($1, $2, $3)
		RETURNING id, user_id, role_id, granted_by, granted_at
	`, req.UserID, role.ID, user.UserID).Scan(&ur.ID, &ur.UserID, &ur.RoleID, &ur.GrantedBy, &ur.GrantedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "user already assigned to this role", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to assign user", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, ur)
}

// RemoveUser removes a user from a role.
func (h *RoleHandler) RemoveUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireSuperuser(w, r) {
		return
	}
	role, ok := h.loadRole(w, r)
	if !ok {
		return
	}

	targetUserID, err := urlParamInt64(r, "userID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID", "bad_request")
		return
	}

	// Don't strand an auto-created admin role with no members.
	if role.IsAutoCreated {
		var memberCount int
		if err := h.DB.QueryRowContext(r.Context(), `
			SELECT COUNT(*) FROM user_roles WHERE role_id = $1
		`, role.ID).Scan(&memberCount); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		if memberCount <= 1 {
			writeError(w, http.StatusBadRequest, "cannot remove the last member of an auto-created admin role", "validation")
			return
		}
	}

	result, err := h.DB.ExecContext(r.Context(), `
		DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2
	`, targetUserID, role.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeError(w, http.StatusNotFound, "user is not assigned to this role", "not_found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// List lists all global roles with their permissions and members. The per-member
// managed roles (backing the member permission toggles) are an implementation
// detail and are hidden. Readable by any authenticated user; ViewerCanManage
// reports whether the caller (a superuser) may manage roles.
func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	roleRows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, name, is_auto_created, created_at
		FROM roles
		WHERE NOT starts_with(name, '`+managedMemberRolePrefix+`')
		ORDER BY name
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	roles, err := scanRoles(roleRows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	if err := h.loadRolePermsAndMembers(r.Context(), roles); err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	su, err := middleware.IsSuperuser(r.Context(), h.DB, currentUser(r).UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "permission check failed", "internal")
		return
	}

	writeJSON(w, http.StatusOK, models.RolesResponse{Items: roles, Count: len(roles), ViewerCanManage: su})
}

// insertRolePermissions inserts the given permission atoms for a role.
func insertRolePermissions(ctx context.Context, tx *sql.Tx, roleID int64, perms []models.PermissionAtomInput) error {
	for _, p := range perms {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO role_permissions (role_id, action, resource, key_project, key_env, key_name)
			VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''))
		`, roleID, p.Action, p.Resource,
			ptrToString(p.KeyProject), ptrToString(p.KeyEnv), ptrToString(p.KeyName))
		if err != nil {
			return err
		}
	}
	return nil
}

// scanRoles reads role rows into a slice and closes the rows. Never returns nil.
func scanRoles(rows *sql.Rows) ([]models.Role, error) {
	defer rows.Close()
	roles := []models.Role{}
	for rows.Next() {
		var role models.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.IsAutoCreated, &role.CreatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

// loadRolePermsAndMembers populates Permissions and Members (with user names
// joined from users) for each role in place.
func (h *RoleHandler) loadRolePermsAndMembers(ctx context.Context, roles []models.Role) error {
	for i := range roles {
		permRows, err := h.DB.QueryContext(ctx, `
			SELECT id, role_id, action, resource, key_project, key_env, key_name
			FROM role_permissions WHERE role_id = $1
		`, roles[i].ID)
		if err != nil {
			return err
		}
		roles[i].Permissions = []models.RolePermission{}
		for permRows.Next() {
			var rp models.RolePermission
			if err := permRows.Scan(&rp.ID, &rp.RoleID, &rp.Action, &rp.Resource, &rp.KeyProject, &rp.KeyEnv, &rp.KeyName); err != nil {
				permRows.Close()
				return err
			}
			roles[i].Permissions = append(roles[i].Permissions, rp)
		}
		permRows.Close()
		if err := permRows.Err(); err != nil {
			return err
		}

		memberRows, err := h.DB.QueryContext(ctx, `
			SELECT ur.id, ur.user_id, ur.role_id, ur.granted_by, ur.granted_at, u.username, u.display_name
			FROM user_roles ur
			JOIN users u ON u.id = ur.user_id
			WHERE ur.role_id = $1
			ORDER BY u.username
		`, roles[i].ID)
		if err != nil {
			return err
		}
		roles[i].Members = []models.UserRole{}
		for memberRows.Next() {
			var ur models.UserRole
			if err := memberRows.Scan(&ur.ID, &ur.UserID, &ur.RoleID, &ur.GrantedBy, &ur.GrantedAt, &ur.Username, &ur.DisplayName); err != nil {
				memberRows.Close()
				return err
			}
			roles[i].Members = append(roles[i].Members, ur)
		}
		memberRows.Close()
		if err := memberRows.Err(); err != nil {
			return err
		}
	}
	return nil
}

// ptrToString converts a *string to a string, returning "" for nil.
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
