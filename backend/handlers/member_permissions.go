package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sort"

	"github.com/brian/config-generation/backend/models"
	"github.com/go-chi/chi/v5"
)

// Per-member permissions are stored as atoms on an auto-managed, per-member
// role named "__member__:<project>:<userID>". A project admin (grant holder)
// edits them; this handler translates between the MemberPermissions shape and
// atoms on that hidden role. The role is never surfaced in the roles listing
// and is edited only through these endpoints.
//
// Templates are project-wide (read/write/delete:project_templates(p)). Value
// access is per environment so a member can be granted, e.g., staging but not
// production:
//   - "Read only"   → read:project_values(p, env)
//   - "Read & write" → write:project_values(p, env) + create:env_values(p, env)
//     (lets the member edit and bootstrap that env's value set; create is env-
//     scoped so it does not grant creating new environments).

const managedMemberRolePrefix = "__member__:"

func managedMemberRoleName(projectName string, userID int64) string {
	return fmt.Sprintf("%s%s:%d", managedMemberRolePrefix, projectName, userID)
}

// templatePermAtom maps a project-wide template capability to its atom.
type templatePermAtom struct {
	action  string
	enabled func(p models.MemberPermissions) bool
	set     func(p *models.MemberPermissions)
}

func templatePermAtoms() []templatePermAtom {
	return []templatePermAtom{
		{models.ActionRead,
			func(p models.MemberPermissions) bool { return p.ReadTemplates },
			func(p *models.MemberPermissions) { p.ReadTemplates = true }},
		{models.ActionWrite,
			func(p models.MemberPermissions) bool { return p.WriteTemplates },
			func(p *models.MemberPermissions) { p.WriteTemplates = true }},
		{models.ActionDelete,
			func(p models.MemberPermissions) bool { return p.DeleteTemplates },
			func(p *models.MemberPermissions) { p.DeleteTemplates = true }},
	}
}

// memberIsMember reports whether userID is a member of the project.
func (h *ProjectMemberHandler) memberIsMember(ctx context.Context, projectID, userID int64) (bool, error) {
	var exists bool
	err := h.DB.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id = $1 AND user_id = $2)`,
		projectID, userID).Scan(&exists)
	return exists, err
}

// projectEnvNames returns the set of environment names in the project.
func (h *ProjectMemberHandler) projectEnvNames(ctx context.Context, projectID int64) (map[string]bool, error) {
	rows, err := h.DB.QueryContext(ctx,
		`SELECT name FROM environments WHERE project_id = $1`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names[n] = true
	}
	return names, rows.Err()
}

// loadMemberPermissions reads the managed role's atoms and assembles them
// into a MemberPermissions struct. Extracted from GetMemberPermissions to
// keep its cognitive complexity within Sonar's limit.
func (h *ProjectMemberHandler) loadMemberPermissions(ctx context.Context, roleName string) (models.MemberPermissions, error) {
	var perms models.MemberPermissions
	rows, err := h.DB.QueryContext(ctx, `
		SELECT rp.action, rp.resource, rp.key_env
		FROM roles r
		JOIN role_permissions rp ON rp.role_id = r.id
		WHERE r.name = $1
	`, roleName)
	if err != nil {
		return perms, err
	}
	defer rows.Close()

	tmplAtoms := templatePermAtoms()
	envAccess := map[string]*models.EnvValueAccess{}
	for rows.Next() {
		var action, resource string
		var keyEnv *string
		if err := rows.Scan(&action, &resource, &keyEnv); err != nil {
			return perms, err
		}
		applyMemberAtomRow(&perms, envAccess, tmplAtoms, action, resource, keyEnv)
	}
	if err := rows.Err(); err != nil {
		return perms, err
	}

	perms.Environments = []models.EnvValueAccess{}
	for _, e := range envAccess {
		perms.Environments = append(perms.Environments, *e)
	}
	sort.Slice(perms.Environments, func(i, j int) bool {
		return perms.Environments[i].Env < perms.Environments[j].Env
	})
	return perms, nil
}

// applyMemberAtomRow folds one role_permissions row into the perms struct or
// the env-access map.
func applyMemberAtomRow(perms *models.MemberPermissions, envAccess map[string]*models.EnvValueAccess, tmplAtoms []templatePermAtom, action, resource string, keyEnv *string) {
	switch resource {
	case models.ResourceProjectTemplates:
		for _, a := range tmplAtoms {
			if a.action == action {
				a.set(perms)
			}
		}
	case models.ResourceProjectValues:
		if keyEnv == nil {
			return
		}
		e := envAccess[*keyEnv]
		if e == nil {
			e = &models.EnvValueAccess{Env: *keyEnv}
			envAccess[*keyEnv] = e
		}
		switch action {
		case models.ActionRead:
			e.Read = true
		case models.ActionWrite:
			e.Read = true
			e.Write = true
		}
	}
}

// GetMemberPermissions returns the member's granted capabilities.
func (h *ProjectMemberHandler) GetMemberPermissions(w http.ResponseWriter, r *http.Request) {
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

	isMember, err := h.memberIsMember(r.Context(), projectID, targetUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	if !isMember {
		writeError(w, http.StatusNotFound, "user is not a member of this project", "not_found")
		return
	}

	perms, err := h.loadMemberPermissions(r.Context(), managedMemberRoleName(projectName, targetUserID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, perms)
}

// validateAndDedupEnvAccess validates each requested env exists and folds the
// list into a map (last entry per env wins). Returns status==0 on success.
func (h *ProjectMemberHandler) validateAndDedupEnvAccess(ctx context.Context, projectID int64, envs []models.EnvValueAccess) (map[string]models.EnvValueAccess, int, string, string) {
	envNames, err := h.projectEnvNames(ctx, projectID)
	if err != nil {
		return nil, http.StatusInternalServerError, msgDatabaseError, "internal"
	}
	envAccess := map[string]models.EnvValueAccess{}
	for _, e := range envs {
		if !envNames[e.Env] {
			return nil, http.StatusBadRequest, fmt.Sprintf("unknown environment %q", e.Env), "validation"
		}
		envAccess[e.Env] = e
	}
	return envAccess, 0, "", ""
}

// upsertManagedMemberRole returns the id of the managed per-member role,
// creating it if needed.
func upsertManagedMemberRole(ctx context.Context, tx *sql.Tx, roleName string) (int64, error) {
	var roleID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = $1`, roleName).Scan(&roleID)
	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO roles (name, is_auto_created) VALUES ($1, false)
			RETURNING id
		`, roleName).Scan(&roleID)
	}
	return roleID, err
}

// rewriteMemberRoleAtoms clears and rewrites the role's atoms from req +
// envAccess.
func rewriteMemberRoleAtoms(ctx context.Context, tx *sql.Tx, roleID int64, projectName string, req models.MemberPermissions, envAccess map[string]models.EnvValueAccess) (int, string, string) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return http.StatusInternalServerError, "failed to clear permissions", "internal"
	}

	insertAtom := func(action, resource, env string) error {
		_, e := tx.ExecContext(ctx, `
			INSERT INTO role_permissions (role_id, action, resource, key_project, key_env)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		`, roleID, action, resource, projectName, env)
		return e
	}

	for _, a := range templatePermAtoms() {
		if !a.enabled(req) {
			continue
		}
		if err := insertAtom(a.action, models.ResourceProjectTemplates, ""); err != nil {
			return http.StatusInternalServerError, msgFailedToSetPerms, "internal"
		}
	}

	for env, access := range envAccess {
		if status, msg, code := insertEnvAccessAtoms(insertAtom, env, access); status != 0 {
			return status, msg, code
		}
	}
	return 0, "", ""
}

// insertEnvAccessAtoms writes the atoms for one env's access level.
func insertEnvAccessAtoms(insertAtom func(action, resource, env string) error, env string, access models.EnvValueAccess) (int, string, string) {
	switch {
	case access.Write:
		// write + create (bootstrap), both env-scoped; read is implied.
		if err := insertAtom(models.ActionWrite, models.ResourceProjectValues, env); err != nil {
			return http.StatusInternalServerError, msgFailedToSetPerms, "internal"
		}
		if err := insertAtom(models.ActionCreate, models.ResourceEnvValues, env); err != nil {
			return http.StatusInternalServerError, msgFailedToSetPerms, "internal"
		}
	case access.Read:
		if err := insertAtom(models.ActionRead, models.ResourceProjectValues, env); err != nil {
			return http.StatusInternalServerError, msgFailedToSetPerms, "internal"
		}
	}
	return 0, "", ""
}

// SetMemberPermissions replaces the member's granted capabilities. It finds or
// creates the per-member managed role, rewrites its atoms, and ensures the
// member is assigned to it.
func (h *ProjectMemberHandler) SetMemberPermissions(w http.ResponseWriter, r *http.Request) {
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

	var req models.MemberPermissions
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidRequestBody, "bad_request")
		return
	}

	isMember, err := h.memberIsMember(r.Context(), projectID, targetUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	if !isMember {
		writeError(w, http.StatusNotFound, "user is not a member of this project", "not_found")
		return
	}

	envAccess, status, msg, code := h.validateAndDedupEnvAccess(r.Context(), projectID, req.Environments)
	if status != 0 {
		writeError(w, status, msg, code)
		return
	}

	actor := currentUser(r)

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, msgDatabaseError, "internal")
		return
	}
	defer tx.Rollback()

	roleName := managedMemberRoleName(projectName, targetUserID)
	roleID, err := upsertManagedMemberRole(r.Context(), tx, roleName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to upsert member role", "internal")
		return
	}

	if status, msg, code := rewriteMemberRoleAtoms(r.Context(), tx, roleID, projectName, req, envAccess); status != 0 {
		writeError(w, status, msg, code)
		return
	}

	// Ensure the member is assigned to the managed role.
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO user_roles (user_id, role_id, granted_by) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, role_id) DO NOTHING
	`, targetUserID, roleID, actor.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to assign member role", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, msgFailedToCommit, "internal")
		return
	}

	writeJSON(w, http.StatusOK, req)
}
