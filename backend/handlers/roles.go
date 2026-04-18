package handlers

import (
	"database/sql"
	"net/http"

	"github.com/brian/config-generation/backend/middleware"
	"github.com/brian/config-generation/backend/models"
	"github.com/go-chi/chi/v5"
)

type RoleHandler struct {
	DB *sql.DB
}

// checkGrantPermission looks up the role's scope (project or global values entry),
// then checks that the user holds the appropriate grant permission.
// Returns the role and true if authorized.
func (h *RoleHandler) checkGrantPermission(w http.ResponseWriter, r *http.Request) (*models.Role, bool) {
	roleID, err := urlParamInt64(r, "roleID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid role ID", "bad_request")
		return nil, false
	}

	var role models.Role
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT r.id, r.name, r.project_id, r.global_values_name, r.is_auto_created, r.created_at
		FROM roles r WHERE r.id = $1
	`, roleID).Scan(&role.ID, &role.Name, &role.ProjectID, &role.GlobalValuesName, &role.IsAutoCreated, &role.CreatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "role not found", "not_found")
		return nil, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return nil, false
	}

	user := currentUser(r)

	if role.GlobalValuesName != nil {
		// Global values scoped role: check grant(global_values, name).
		allowed, err := middleware.CheckPermission(r.Context(), h.DB, user.UserID, models.PermissionRequirement{
			Action:   models.ActionGrant,
			Resource: models.ResourceGlobalValues,
			KeyName:  *role.GlobalValuesName,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "permission check failed", "internal")
			return nil, false
		}
		if !allowed {
			writeError(w, http.StatusForbidden, "insufficient permissions", "forbidden")
			return nil, false
		}
		return &role, true
	}

	if role.ProjectID != nil {
		// Project scoped role: check grant(project).
		var projectName string
		err = h.DB.QueryRowContext(r.Context(), `SELECT name FROM projects WHERE id = $1`, *role.ProjectID).Scan(&projectName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return nil, false
		}

		allowed, err := middleware.CheckPermission(r.Context(), h.DB, user.UserID, models.PermissionRequirement{
			Action:     models.ActionGrant,
			KeyProject: projectName,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "permission check failed", "internal")
			return nil, false
		}
		if !allowed {
			writeError(w, http.StatusForbidden, "insufficient permissions", "forbidden")
			return nil, false
		}
		return &role, true
	}

	writeError(w, http.StatusForbidden, "system-level role management not yet supported", "forbidden")
	return nil, false
}

// Create creates a new custom role within a project.
func (h *RoleHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "projectName")

	// Resolve project.
	var projectID int64
	err := h.DB.QueryRowContext(r.Context(), `SELECT id FROM projects WHERE name = $1`, projectName).Scan(&projectID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
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
		INSERT INTO roles (name, project_id, is_auto_created) VALUES ($1, $2, false)
		RETURNING id, name, project_id, global_values_name, is_auto_created, created_at
	`, req.Name, projectID).Scan(&role.ID, &role.Name, &role.ProjectID, &role.GlobalValuesName, &role.IsAutoCreated, &role.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "role name already exists for this project", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create role", "internal")
		return
	}

	// Insert permission atoms.
	for _, p := range req.Permissions {
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO role_permissions (role_id, action, resource, key_project, key_env, key_name)
			VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''))
		`, role.ID, p.Action, p.Resource,
			ptrToString(p.KeyProject), ptrToString(p.KeyEnv), ptrToString(p.KeyName))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create role permissions", "internal")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, role)
}

// EditPermissions replaces all permissions for a custom role.
func (h *RoleHandler) EditPermissions(w http.ResponseWriter, r *http.Request) {
	role, ok := h.checkGrantPermission(w, r)
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

	// Delete existing permissions.
	_, err = tx.ExecContext(r.Context(), `DELETE FROM role_permissions WHERE role_id = $1`, role.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear permissions", "internal")
		return
	}

	// Insert new permissions.
	for _, p := range req.Permissions {
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO role_permissions (role_id, action, resource, key_project, key_env, key_name)
			VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''))
		`, role.ID, p.Action, p.Resource,
			ptrToString(p.KeyProject), ptrToString(p.KeyEnv), ptrToString(p.KeyName))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create role permissions", "internal")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Delete deletes a custom role and revokes all member assignments.
func (h *RoleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	role, ok := h.checkGrantPermission(w, r)
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

	_, err = tx.ExecContext(r.Context(), `DELETE FROM user_roles WHERE role_id = $1`, role.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke role assignments", "internal")
		return
	}
	_, err = tx.ExecContext(r.Context(), `DELETE FROM role_permissions WHERE role_id = $1`, role.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete role permissions", "internal")
		return
	}
	_, err = tx.ExecContext(r.Context(), `DELETE FROM roles WHERE id = $1`, role.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete role", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AssignUser assigns a user to a role.
func (h *RoleHandler) AssignUser(w http.ResponseWriter, r *http.Request) {
	role, ok := h.checkGrantPermission(w, r)
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
	role, ok := h.checkGrantPermission(w, r)
	if !ok {
		return
	}

	targetUserID, err := urlParamInt64(r, "userID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID", "bad_request")
		return
	}

	// Prevent removing the last project_admin member.
	if role.IsAutoCreated {
		var memberCount int
		err = h.DB.QueryRowContext(r.Context(), `
			SELECT COUNT(*) FROM user_roles WHERE role_id = $1
		`, role.ID).Scan(&memberCount)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		if memberCount <= 1 {
			writeError(w, http.StatusBadRequest, "cannot remove the last member of the project admin role", "validation")
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

// ListForProject lists all roles for a project with their permissions and members.
func (h *RoleHandler) ListForProject(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "projectName")

	var projectID int64
	err := h.DB.QueryRowContext(r.Context(), `SELECT id FROM projects WHERE name = $1`, projectName).Scan(&projectID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	// Fetch roles.
	roleRows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, name, project_id, global_values_name, is_auto_created, created_at
		FROM roles WHERE project_id = $1 ORDER BY name
	`, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer roleRows.Close()

	var roles []models.Role
	for roleRows.Next() {
		var role models.Role
		if err := roleRows.Scan(&role.ID, &role.Name, &role.ProjectID, &role.GlobalValuesName, &role.IsAutoCreated, &role.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		roles = append(roles, role)
	}
	if err := roleRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	// Fetch permissions and members for each role.
	for i := range roles {
		permRows, err := h.DB.QueryContext(r.Context(), `
			SELECT id, role_id, action, resource, key_project, key_env, key_name
			FROM role_permissions WHERE role_id = $1
		`, roles[i].ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		roles[i].Permissions = []models.RolePermission{}
		for permRows.Next() {
			var rp models.RolePermission
			if err := permRows.Scan(&rp.ID, &rp.RoleID, &rp.Action, &rp.Resource, &rp.KeyProject, &rp.KeyEnv, &rp.KeyName); err != nil {
				permRows.Close()
				writeError(w, http.StatusInternalServerError, "database error", "internal")
				return
			}
			roles[i].Permissions = append(roles[i].Permissions, rp)
		}
		permRows.Close()

		memberRows, err := h.DB.QueryContext(r.Context(), `
			SELECT id, user_id, role_id, granted_by, granted_at
			FROM user_roles WHERE role_id = $1
		`, roles[i].ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		roles[i].Members = []models.UserRole{}
		for memberRows.Next() {
			var ur models.UserRole
			if err := memberRows.Scan(&ur.ID, &ur.UserID, &ur.RoleID, &ur.GrantedBy, &ur.GrantedAt); err != nil {
				memberRows.Close()
				writeError(w, http.StatusInternalServerError, "database error", "internal")
				return
			}
			roles[i].Members = append(roles[i].Members, ur)
		}
		memberRows.Close()
	}

	if roles == nil {
		roles = []models.Role{}
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.Role]{Items: roles, Count: len(roles)})
}

// ListForGlobalValues lists all roles scoped to a global values entry with their permissions and members.
func (h *RoleHandler) ListForGlobalValues(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	// Verify the global values entry exists.
	var exists bool
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM global_values WHERE name = $1)
	`, name).Scan(&exists)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "global values entry not found", "not_found")
		return
	}

	roleRows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, name, project_id, global_values_name, is_auto_created, created_at
		FROM roles WHERE global_values_name = $1 ORDER BY name
	`, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer roleRows.Close()

	var roles []models.Role
	for roleRows.Next() {
		var role models.Role
		if err := roleRows.Scan(&role.ID, &role.Name, &role.ProjectID, &role.GlobalValuesName, &role.IsAutoCreated, &role.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		roles = append(roles, role)
	}
	if err := roleRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	// Fetch permissions and members for each role.
	for i := range roles {
		permRows, err := h.DB.QueryContext(r.Context(), `
			SELECT id, role_id, action, resource, key_project, key_env, key_name
			FROM role_permissions WHERE role_id = $1
		`, roles[i].ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		roles[i].Permissions = []models.RolePermission{}
		for permRows.Next() {
			var rp models.RolePermission
			if err := permRows.Scan(&rp.ID, &rp.RoleID, &rp.Action, &rp.Resource, &rp.KeyProject, &rp.KeyEnv, &rp.KeyName); err != nil {
				permRows.Close()
				writeError(w, http.StatusInternalServerError, "database error", "internal")
				return
			}
			roles[i].Permissions = append(roles[i].Permissions, rp)
		}
		permRows.Close()

		memberRows, err := h.DB.QueryContext(r.Context(), `
			SELECT id, user_id, role_id, granted_by, granted_at
			FROM user_roles WHERE role_id = $1
		`, roles[i].ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		roles[i].Members = []models.UserRole{}
		for memberRows.Next() {
			var ur models.UserRole
			if err := memberRows.Scan(&ur.ID, &ur.UserID, &ur.RoleID, &ur.GrantedBy, &ur.GrantedAt); err != nil {
				memberRows.Close()
				writeError(w, http.StatusInternalServerError, "database error", "internal")
				return
			}
			roles[i].Members = append(roles[i].Members, ur)
		}
		memberRows.Close()
	}

	if roles == nil {
		roles = []models.Role{}
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.Role]{Items: roles, Count: len(roles)})
}

// ptrToString converts a *string to a string, returning "" for nil.
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
