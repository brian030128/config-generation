package middleware

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/brian/config-generation/backend/models"
	"github.com/go-chi/chi/v5"
)

// effectivePermission is one row from the user's effective permission set.
type effectivePermission struct {
	Action     string
	Resource   string
	KeyProject *string
	KeyEnv     *string
	KeyName    *string
}

// loadEffectivePermissions queries all permission atoms for a user by joining
// user_roles with role_permissions, plus atoms synthesized from two membership
// concepts that are not stored as role_permissions:
//   - project membership → read:project(p) (the only source of read:project).
//   - env-admin grants → read:project(p), create:env_values(p, env) and
//     delete:project_values(p, env) for the granted environment. create implies
//     write implies read on that env's values (see satisfies), so these three
//     atoms give full control of the env's value sets plus env deletion.
func loadEffectivePermissions(ctx context.Context, db *sql.DB, userID int64) ([]effectivePermission, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT rp.action, rp.resource, rp.key_project, rp.key_env, rp.key_name
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		WHERE ur.user_id = $1
		UNION ALL
		SELECT 'read', 'project', p.name, NULL, NULL
		FROM project_members pm
		JOIN projects p ON p.id = pm.project_id
		WHERE pm.user_id = $1
		UNION ALL
		SELECT 'read', 'project', p.name, NULL, NULL
		FROM env_admins ea
		JOIN environments e ON e.id = ea.environment_id
		JOIN projects p ON p.id = e.project_id
		WHERE ea.user_id = $1
		UNION ALL
		SELECT 'create', 'env_values', p.name, e.name, NULL
		FROM env_admins ea
		JOIN environments e ON e.id = ea.environment_id
		JOIN projects p ON p.id = e.project_id
		WHERE ea.user_id = $1
		UNION ALL
		SELECT 'delete', 'project_values', p.name, e.name, NULL
		FROM env_admins ea
		JOIN environments e ON e.id = ea.environment_id
		JOIN projects p ON p.id = e.project_id
		WHERE ea.user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []effectivePermission
	for rows.Next() {
		var p effectivePermission
		if err := rows.Scan(&p.Action, &p.Resource, &p.KeyProject, &p.KeyEnv, &p.KeyName); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// matchesKey returns true if the granted key value satisfies the required key.
// granted "*" matches anything; required "" means slot not applicable (always matches).
func matchesKey(granted *string, required string) bool {
	if required == "" {
		return true
	}
	if granted == nil {
		return false
	}
	if *granted == "*" {
		return true
	}
	return *granted == required
}

// satisfies checks whether a single granted permission satisfies a requirement,
// accounting for wildcard matching and implication rules.
func satisfies(granted effectivePermission, req models.PermissionRequirement) bool {
	// Direct match: same action + resource, keys match.
	if granted.Action == req.Action && granted.Resource == req.Resource {
		return matchesKey(granted.KeyProject, req.KeyProject) &&
			matchesKey(granted.KeyEnv, req.KeyEnv) &&
			matchesKey(granted.KeyName, req.KeyName)
	}

	// Write implies read (same resource).
	if req.Action == models.ActionRead && granted.Action == models.ActionWrite && granted.Resource == req.Resource {
		return matchesKey(granted.KeyProject, req.KeyProject) &&
			matchesKey(granted.KeyEnv, req.KeyEnv) &&
			matchesKey(granted.KeyName, req.KeyName)
	}

	// create:env_values(project, env) implies write:project_values(project, env).
	// The env key is matched so an env-scoped create (an env-admin's grant) only
	// confers write on that env, while a project admin's create:env_values(p, *)
	// still confers write across every env.
	if req.Action == models.ActionWrite && req.Resource == models.ResourceProjectValues &&
		granted.Action == models.ActionCreate && granted.Resource == models.ResourceEnvValues {
		return matchesKey(granted.KeyProject, req.KeyProject) &&
			matchesKey(granted.KeyEnv, req.KeyEnv)
	}

	// create:env_values(project, env) implies read:project_values(project, env) (transitive via write→read).
	if req.Action == models.ActionRead && req.Resource == models.ResourceProjectValues &&
		granted.Action == models.ActionCreate && granted.Resource == models.ResourceEnvValues {
		return matchesKey(granted.KeyProject, req.KeyProject) &&
			matchesKey(granted.KeyEnv, req.KeyEnv)
	}

	return false
}

// HasPermission checks if any of the user's effective permissions satisfy the requirement.
func HasPermission(perms []effectivePermission, req models.PermissionRequirement) bool {
	for _, p := range perms {
		if satisfies(p, req) {
			return true
		}
	}
	return false
}

// KeyExtractor resolves a permission key value from the request at runtime.
type KeyExtractor func(r *http.Request) string

// URLParam returns a KeyExtractor that reads a chi URL parameter.
func URLParam(name string) KeyExtractor {
	return func(r *http.Request) string {
		return chi.URLParam(r, name)
	}
}

// Static returns a KeyExtractor that always returns the given value.
func Static(val string) KeyExtractor {
	return func(r *http.Request) string {
		return val
	}
}

// RequirePermission returns middleware that checks the authenticated user holds
// the specified permission. Key values are resolved at request time via extractors.
// Pass nil for key extractors that are not applicable for the resource.
func isSuperuser(ctx context.Context, db *sql.DB, userID int64) (bool, error) {
	var su bool
	err := db.QueryRowContext(ctx, "SELECT superuser FROM users WHERE id = $1", userID).Scan(&su)
	if err != nil {
		return false, err
	}
	return su, nil
}

func RequirePermission(
	db *sql.DB,
	action string,
	resource string,
	projectKey KeyExtractor,
	envKey KeyExtractor,
	nameKey KeyExtractor,
) func(http.Handler) http.Handler {
	cfg := permissionGateConfig{
		action:     action,
		resource:   resource,
		projectKey: projectKey,
		envKey:     envKey,
		nameKey:    nameKey,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			servePermissionGate(db, cfg, next, w, r)
		})
	}
}

// permissionGateConfig collects the static config a RequirePermission gate is
// built with so servePermissionGate stays within Sonar's parameter limit.
type permissionGateConfig struct {
	action, resource                   string
	projectKey, envKey, nameKey KeyExtractor
}

// servePermissionGate is the body of the RequirePermission middleware,
// extracted from the closure-within-closure so its cognitive complexity stays
// within Sonar's limit. Behavior is unchanged.
func servePermissionGate(
	db *sql.DB,
	cfg permissionGateConfig,
	next http.Handler,
	w http.ResponseWriter,
	r *http.Request,
) {
	user := UserFromContext(r.Context())

	su, err := isSuperuser(r.Context(), db, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check user", "internal")
		return
	}
	if su {
		next.ServeHTTP(w, r)
		return
	}

	perms, err := loadEffectivePermissions(r.Context(), db, user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load permissions", "internal")
		return
	}

	req := models.PermissionRequirement{
		Action:   cfg.action,
		Resource: cfg.resource,
	}
	if cfg.projectKey != nil {
		req.KeyProject = cfg.projectKey(r)
	}
	if cfg.envKey != nil {
		req.KeyEnv = cfg.envKey(r)
	}
	if cfg.nameKey != nil {
		req.KeyName = cfg.nameKey(r)
	}

	if !HasPermission(perms, req) {
		writeError(w, http.StatusForbidden, "insufficient permissions", "forbidden")
		return
	}

	next.ServeHTTP(w, r)
}

// CheckPermission is a helper for handlers that need to check permissions
// programmatically (e.g., role handlers that must look up the project first).
func CheckPermission(ctx context.Context, db *sql.DB, userID int64, req models.PermissionRequirement) (bool, error) {
	su, err := isSuperuser(ctx, db, userID)
	if err != nil {
		return false, err
	}
	if su {
		return true, nil
	}

	perms, err := loadEffectivePermissions(ctx, db, userID)
	if err != nil {
		return false, err
	}
	return HasPermission(perms, req), nil
}

// IsSuperuser reports whether the user has the superuser flag set.
func IsSuperuser(ctx context.Context, db *sql.DB, userID int64) (bool, error) {
	return isSuperuser(ctx, db, userID)
}

// EffectivePermissionSet is a user's full set of effective permission atoms,
// loaded once. Use Can to test requirements without re-querying — useful for
// filtering a list (e.g. scoping projects to those the caller can read) where
// per-item CheckPermission calls would be N+1 queries.
type EffectivePermissionSet struct {
	perms []effectivePermission
}

// Can reports whether the set satisfies the requirement, applying the same
// wildcard and implication rules as the route middleware.
func (s EffectivePermissionSet) Can(req models.PermissionRequirement) bool {
	return HasPermission(s.perms, req)
}

// LoadEffectivePermissions loads all effective permission atoms for a user in a
// single query (including read:project atoms synthesized from membership).
func LoadEffectivePermissions(ctx context.Context, db *sql.DB, userID int64) (EffectivePermissionSet, error) {
	perms, err := loadEffectivePermissions(ctx, db, userID)
	if err != nil {
		return EffectivePermissionSet{}, err
	}
	return EffectivePermissionSet{perms: perms}, nil
}
