package handlers

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/brian/config-generation/backend/middleware"
	"github.com/brian/config-generation/backend/models"
	"github.com/go-chi/chi/v5"
)

// ProjectMemberHandler manages a project's membership. Membership is the source
// of read:project(name); the project admin (grant holder) adds and removes
// members. Managing members is gated in router.go (read:project to list,
// grant(p) to add/remove).
type ProjectMemberHandler struct {
	DB *sql.DB
}

// resolveProjectID looks up a project's ID by name, writing a 404 and returning
// ok=false if it does not exist.
func (h *ProjectMemberHandler) resolveProjectID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	var id int64
	err := h.DB.QueryRowContext(r.Context(), `SELECT id FROM projects WHERE name = $1`, name).Scan(&id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, msgProjectNotFound, "not_found")
		return 0, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return 0, false
	}
	return id, true
}

// ListMembers returns every member of the project.
func (h *ProjectMemberHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "projectName")
	projectID, ok := h.resolveProjectID(w, r, projectName)
	if !ok {
		return
	}

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT pm.user_id, u.username, u.display_name, pm.added_by, pm.added_at
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id = $1
		ORDER BY pm.added_at
	`, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer rows.Close()

	members := []models.ProjectMember{}
	for rows.Next() {
		var m models.ProjectMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.DisplayName, &m.AddedBy, &m.AddedAt); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	// Whether the caller may manage members (add/remove) — i.e. holds grant(p).
	// Same check the add/remove routes enforce; superusers pass via the bypass.
	canManage, err := middleware.CheckPermission(r.Context(), h.DB, currentUser(r).UserID, models.PermissionRequirement{
		Action:     models.ActionGrant,
		KeyProject: projectName,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "permission check failed", "internal")
		return
	}

	writeJSON(w, http.StatusOK, models.ProjectMembersResponse{
		Items:           members,
		Count:           len(members),
		ViewerCanManage: canManage,
	})
}

// AddMember adds a user to the project, granting them read:project.
func (h *ProjectMemberHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "projectName")
	projectID, ok := h.resolveProjectID(w, r, projectName)
	if !ok {
		return
	}

	var req models.AddProjectMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if req.UserID == 0 {
		writeError(w, http.StatusBadRequest, "user_id is required", "validation")
		return
	}

	// Verify the target user exists for a clean error (the FK would otherwise
	// surface as a generic 500).
	var exists bool
	if err := h.DB.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, req.UserID).Scan(&exists); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "user not found", "not_found")
		return
	}

	actor := currentUser(r)
	var m models.ProjectMember
	err := h.DB.QueryRowContext(r.Context(), `
		WITH inserted AS (
			INSERT INTO project_members (project_id, user_id, added_by) VALUES ($1, $2, $3)
			RETURNING user_id, added_by, added_at
		)
		SELECT i.user_id, u.username, u.display_name, i.added_by, i.added_at
		FROM inserted i JOIN users u ON u.id = i.user_id
	`, projectID, req.UserID, actor.UserID).Scan(&m.UserID, &m.Username, &m.DisplayName, &m.AddedBy, &m.AddedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "user is already a member of this project", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add member", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, m)
}

// ensureNotLastProjectAdmin returns a non-zero HTTP status with message and
// code when removing targetUserID would leave the project's auto-created admin
// role with zero members. A 0 status means the removal is safe to proceed.
// Extracted from RemoveMember to keep its cognitive complexity within limits.
func (h *ProjectMemberHandler) ensureNotLastProjectAdmin(ctx context.Context, projectName string, targetUserID int64) (int, string, string) {
	var adminRoleID int64
	err := h.DB.QueryRowContext(ctx,
		`SELECT id FROM roles WHERE name = $1 AND is_auto_created = true LIMIT 1`,
		projectName+"_project_admin").Scan(&adminRoleID)
	if err == sql.ErrNoRows {
		return 0, "", ""
	}
	if err != nil {
		return http.StatusInternalServerError, msgDatabaseError, "internal"
	}
	var isAdmin bool
	var adminCount int
	if err := h.DB.QueryRowContext(ctx, `
		SELECT
			EXISTS(SELECT 1 FROM user_roles WHERE role_id = $1 AND user_id = $2),
			(SELECT COUNT(*) FROM user_roles WHERE role_id = $1)
	`, adminRoleID, targetUserID).Scan(&isAdmin, &adminCount); err != nil {
		return http.StatusInternalServerError, msgDatabaseError, "internal"
	}
	if isAdmin && adminCount <= 1 {
		return http.StatusBadRequest, "cannot remove the last project admin", "validation"
	}
	return 0, "", ""
}

// RemoveMember removes a user from the project. It also revokes the user's
// project-scoped role assignments so no permissions remain without membership.
// Removing the sole member of the project_admin role is refused to avoid
// locking the project out of administration.
func (h *ProjectMemberHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "projectName")
	projectID, ok := h.resolveProjectID(w, r, projectName)
	if !ok {
		return
	}

	targetUserID, err := urlParamInt64(r, "userID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID", "bad_request")
		return
	}

	// Guard: refuse to remove the sole admin (would leave the project with no
	// admin). See ensureNotLastProjectAdmin for the role-resolution details.
	if status, msg, code := h.ensureNotLastProjectAdmin(r.Context(), projectName, targetUserID); status != 0 {
		writeError(w, status, msg, code)
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tx.Rollback()

	// Clean up the user's per-member managed role for this project (the
	// fine-grained capabilities a project admin granted them via the member
	// permissions editor). Named global roles are managed separately (superuser)
	// and are intentionally left untouched.
	managedRole := managedMemberRoleName(projectName, targetUserID)
	for _, q := range []string{
		`DELETE FROM user_roles WHERE role_id IN (SELECT id FROM roles WHERE name = $1)`,
		`DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM roles WHERE name = $1)`,
		`DELETE FROM roles WHERE name = $1`,
	} {
		if _, err = tx.ExecContext(r.Context(), q, managedRole); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
	}

	result, err := tx.ExecContext(r.Context(),
		`DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`,
		projectID, targetUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeError(w, http.StatusNotFound, "user is not a member of this project", "not_found")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, msgFailedToCommit, "internal")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
