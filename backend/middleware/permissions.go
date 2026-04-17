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

// loadEffectivePermissions queries all permission atoms for a user
// by joining user_roles with role_permissions.
func loadEffectivePermissions(ctx context.Context, db *sql.DB, userID int64) ([]effectivePermission, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT rp.action, rp.resource, rp.key_project, rp.key_env, rp.key_name
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		WHERE ur.user_id = $1
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

	// create:env_values(project) implies write:project_values(project, *).
	if req.Action == models.ActionWrite && req.Resource == models.ResourceProjectValues &&
		granted.Action == models.ActionCreate && granted.Resource == models.ResourceEnvValues {
		return matchesKey(granted.KeyProject, req.KeyProject)
	}

	// create:env_values(project) implies read:project_values(project, *) (transitive via write→read).
	if req.Action == models.ActionRead && req.Resource == models.ResourceProjectValues &&
		granted.Action == models.ActionCreate && granted.Resource == models.ResourceEnvValues {
		return matchesKey(granted.KeyProject, req.KeyProject)
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
func RequirePermission(
	db *sql.DB,
	action string,
	resource string,
	projectKey KeyExtractor,
	envKey KeyExtractor,
	nameKey KeyExtractor,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())

			perms, err := loadEffectivePermissions(r.Context(), db, user.UserID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to load permissions", "internal")
				return
			}

			req := models.PermissionRequirement{
				Action:   action,
				Resource: resource,
			}
			if projectKey != nil {
				req.KeyProject = projectKey(r)
			}
			if envKey != nil {
				req.KeyEnv = envKey(r)
			}
			if nameKey != nil {
				req.KeyName = nameKey(r)
			}

			if !HasPermission(perms, req) {
				writeError(w, http.StatusForbidden, "insufficient permissions", "forbidden")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CheckPermission is a helper for handlers that need to check permissions
// programmatically (e.g., role handlers that must look up the project first).
func CheckPermission(ctx context.Context, db *sql.DB, userID int64, req models.PermissionRequirement) (bool, error) {
	perms, err := loadEffectivePermissions(ctx, db, userID)
	if err != nil {
		return false, err
	}
	return HasPermission(perms, req), nil
}
