package handlers

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/brian/config-generation/backend/middleware"
	"github.com/brian/config-generation/backend/models"
	"github.com/go-chi/chi/v5"
)

// EnvAdminHandler manages an environment's admins. An env-admin has full control
// of the environment's value sets (create/read/write/delete), can delete the
// environment, and can grant env-admin to other users (self-propagating). The
// synthesized permission atoms live in middleware.loadEffectivePermissions;
// this handler only manages the env_admins rows.
//
// Add/Remove authorization is enforced in-handler (env-scoped self-propagation
// is not expressible as route middleware): allowed for superusers, project
// admins (grant(project)), and existing env-admins of the same environment.
type EnvAdminHandler struct {
	DB *sql.DB
}

// resolveEnv looks up the environment id for (projectName, envName), writing a
// 404 and returning ok=false if either is missing.
func (h *EnvAdminHandler) resolveEnv(w http.ResponseWriter, r *http.Request, projectName, envName string) (int64, bool) {
	var envID int64
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT e.id FROM environments e
		JOIN projects p ON p.id = e.project_id
		WHERE p.name = $1 AND e.name = $2
	`, projectName, envName).Scan(&envID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "environment not found", "not_found")
		return 0, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return 0, false
	}
	return envID, true
}

// canManage reports whether the user may add/remove admins for this environment.
func (h *EnvAdminHandler) canManage(ctx context.Context, userID, envID int64, projectName string) (bool, error) {
	su, err := middleware.IsSuperuser(ctx, h.DB, userID)
	if err != nil {
		return false, err
	}
	if su {
		return true, nil
	}

	// Project admins (grant(project)) can manage any env's admins.
	grant, err := middleware.CheckPermission(ctx, h.DB, userID, models.PermissionRequirement{
		Action:     models.ActionGrant,
		KeyProject: projectName,
	})
	if err != nil {
		return false, err
	}
	if grant {
		return true, nil
	}

	// Existing env-admins can grant env-admin to others (self-propagating).
	var isAdmin bool
	err = h.DB.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM env_admins WHERE environment_id = $1 AND user_id = $2)`,
		envID, userID).Scan(&isAdmin)
	return isAdmin, err
}

// List returns every admin of the environment. Gated read:project on the route.
func (h *EnvAdminHandler) List(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "projectName")
	envName := chi.URLParam(r, "envName")
	envID, ok := h.resolveEnv(w, r, projectName, envName)
	if !ok {
		return
	}

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT ea.user_id, u.username, u.display_name, ea.granted_by, ea.granted_at
		FROM env_admins ea
		JOIN users u ON u.id = ea.user_id
		WHERE ea.environment_id = $1
		ORDER BY ea.granted_at
	`, envID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer rows.Close()

	admins := []models.EnvAdmin{}
	for rows.Next() {
		var a models.EnvAdmin
		if err := rows.Scan(&a.UserID, &a.Username, &a.DisplayName, &a.GrantedBy, &a.GrantedAt); err != nil {
			writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
			return
		}
		admins = append(admins, a)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.EnvAdmin]{Items: admins, Count: len(admins)})
}

// resolveAndAuth resolves the (projectName, envName) URL params to an envID
// and checks that the current user may manage the env's admins (via canManage).
// On error it writes the standard 404/403/500 response and returns ok=false.
func (h *EnvAdminHandler) resolveAndAuth(w http.ResponseWriter, r *http.Request) (envID, actorID int64, ok bool) {
	projectName := chi.URLParam(r, "projectName")
	envName := chi.URLParam(r, "envName")
	envID, ok = h.resolveEnv(w, r, projectName, envName)
	if !ok {
		return 0, 0, false
	}

	actor := currentUser(r)
	allowed, err := h.canManage(r.Context(), actor.UserID, envID, projectName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "permission check failed", "internal")
		return 0, 0, false
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "insufficient permissions", "forbidden")
		return 0, 0, false
	}
	return envID, actor.UserID, true
}

// Add grants env-admin on the environment to a user.
func (h *EnvAdminHandler) Add(w http.ResponseWriter, r *http.Request) {
	envID, actorID, ok := h.resolveAndAuth(w, r)
	if !ok {
		return
	}

	var req models.AddEnvAdminRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}
	if req.UserID == 0 {
		writeError(w, http.StatusBadRequest, "user_id is required", "validation")
		return
	}

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

	var a models.EnvAdmin
	err := h.DB.QueryRowContext(r.Context(), `
		WITH inserted AS (
			INSERT INTO env_admins (environment_id, user_id, granted_by) VALUES ($1, $2, $3)
			RETURNING user_id, granted_by, granted_at
		)
		SELECT i.user_id, u.username, u.display_name, i.granted_by, i.granted_at
		FROM inserted i JOIN users u ON u.id = i.user_id
	`, envID, req.UserID, actorID).Scan(&a.UserID, &a.Username, &a.DisplayName, &a.GrantedBy, &a.GrantedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "user is already an admin of this environment", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add env admin", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, a)
}

// Remove revokes env-admin on the environment from a user. The sole remaining
// admin cannot be removed (so an environment is never left adminless).
func (h *EnvAdminHandler) Remove(w http.ResponseWriter, r *http.Request) {
	envID, _, ok := h.resolveAndAuth(w, r)
	if !ok {
		return
	}

	targetUserID, err := urlParamInt64(r, "userID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID", "bad_request")
		return
	}

	// Guard: refuse to remove the sole remaining admin.
	var isAdmin bool
	var adminCount int
	if err := h.DB.QueryRowContext(r.Context(), `
		SELECT
			EXISTS(SELECT 1 FROM env_admins WHERE environment_id = $1 AND user_id = $2),
			(SELECT COUNT(*) FROM env_admins WHERE environment_id = $1)
	`, envID, targetUserID).Scan(&isAdmin, &adminCount); err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	if isAdmin && adminCount <= 1 {
		writeError(w, http.StatusBadRequest, "cannot remove the last admin of the environment", "validation")
		return
	}

	result, err := h.DB.ExecContext(r.Context(),
		`DELETE FROM env_admins WHERE environment_id = $1 AND user_id = $2`,
		envID, targetUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeError(w, http.StatusNotFound, "user is not an admin of this environment", "not_found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
