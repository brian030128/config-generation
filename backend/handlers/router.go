package handlers

import (
	"database/sql"
	"net/http"

	"github.com/brian/config-generation/backend/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Route path literals — extracted to satisfy go:S1192.
const (
	pathTemplates      = "/templates"
	pathVersions       = "/versions"
	pathVersionID      = "/{versionID}"
	pathEnvironments   = "/environments"
	pathTemplateByName = "/templates/{templateName}"
	pathEnvValues      = "/envs/{envName}/values"
)

func NewRouter(db *sql.DB, jwtSecret []byte) chi.Router {
	return NewRouterWithAuthConfig(db, DefaultAuthConfig(jwtSecret))
}

func NewRouterWithAuthConfig(db *sql.DB, authConfig AuthConfig) chi.Router {
	r := chi.NewRouter()

	// Prometheus metrics middleware — wraps all routes.
	r.Use(middleware.PrometheusMetrics)

	// ── Public routes (no JWT) ─────────────────────────────────────────────

	// Liveness probe: always 200 if the process is running.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Readiness probe: 200 only when the DB is reachable.
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Prometheus scrape endpoint.
	r.Handle("/metrics", promhttp.Handler())

	auth := &AuthHandler{DB: db, Config: authConfig}
	r.Route("/api/auth", func(r chi.Router) {
		r.Get("/config", auth.ConfigResponse)
		r.Get("/me", auth.Me)
		r.Post("/session", auth.Session)
		r.Post("/logout", auth.Logout)
		r.Post("/register", auth.Register)
		r.Post("/login", auth.Login)
		r.Get("/oidc/login", auth.OIDCLogin)
		r.Get("/oidc/callback", auth.OIDCCallback)
	})

	proj := &ProjectHandler{DB: db}
	pm := &ProjectMemberHandler{DB: db}
	users := &UserHandler{DB: db}
	env := &EnvironmentHandler{DB: db}
	envadm := &EnvAdminHandler{DB: db}
	tmpl := &TemplateHandler{DB: db}
	vals := &ValuesHandler{DB: db}
	gv := &GlobalValuesHandler{DB: db}
	role := &RoleHandler{DB: db}
	pr := &PullRequestHandler{DB: db}
	deploy := &DeploymentHandler{DB: db}

	perm := middleware.RequirePermission
	param := middleware.URLParam

	// Protected routes: JWT required.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(authConfig.JWTSecret, authConfig.SessionCookieName))
		r.Use(middleware.CSRFProtection(csrfCookieName))

		r.Route("/api", func(r chi.Router) {

			// --- Users (directory search; powers the member picker) ---
			r.Get("/users", users.Search)

			// --- Projects ---
			r.Route("/projects", func(r chi.Router) {
				r.With(perm(db, "create", "project", nil, nil, nil)).
					Post("/", proj.Create)
				r.Get("/", proj.List)

				r.Route("/{projectName}", func(r chi.Router) {
					r.With(perm(db, "read", "project", param("projectName"), nil, nil)).
						Get("/", proj.Get)
					r.With(perm(db, "delete", "project", param("projectName"), nil, nil)).
						Delete("/", proj.Delete)

					// --- Members (membership grants read:project) ---
					r.Route("/members", func(r chi.Router) {
						r.With(perm(db, "read", "project", param("projectName"), nil, nil)).
							Get("/", pm.ListMembers)
						r.With(perm(db, "grant", "", param("projectName"), nil, nil)).
							Post("/", pm.AddMember)
						r.With(perm(db, "grant", "", param("projectName"), nil, nil)).
							Delete("/{userID}", pm.RemoveMember)

						// Per-member capability toggles (project admin grants
						// fine-grained permissions to a member).
						r.With(perm(db, "grant", "", param("projectName"), nil, nil)).
							Get("/{userID}/permissions", pm.GetMemberPermissions)
						r.With(perm(db, "grant", "", param("projectName"), nil, nil)).
							Put("/{userID}/permissions", pm.SetMemberPermissions)
					})

					// --- Templates ---
					r.Route(pathTemplates, func(r chi.Router) {
						r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
							Get("/", tmpl.ListForProject)
						// Authoring (create/edit/delete) goes through the workspace, not here.

						r.Route("/{templateName}", func(r chi.Router) {
							r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
								Get("/", tmpl.GetLatest)
							r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
								Get("/variables", tmpl.Variables)

							r.Route(pathVersions, func(r chi.Router) {
								r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
									Get("/", tmpl.ListVersions)
								r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
									Get(pathVersionID, tmpl.GetVersion)
							})
						})
					})

					// --- Project-level variables (union of all templates) ---
					r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
						Get("/variables", tmpl.ProjectVariables)

					// --- Values per (project, env) — reads only; writes go through the workspace ---
					r.Route("/envs/{envName}", func(r chi.Router) {
						r.Route("/values", func(r chi.Router) {
							r.With(perm(db, "read", "project_values", param("projectName"), param("envName"), nil)).
								Get("/", vals.GetLatest)

							r.Route(pathVersions, func(r chi.Router) {
								r.With(perm(db, "read", "project_values", param("projectName"), param("envName"), nil)).
									Get(pathVersionID, vals.GetVersion)
							})
						})

						// --- Deployments ---
						r.Route("/deploy", func(r chi.Router) {
							r.Post("/preview", deploy.Preview)
							r.Post("/", deploy.Execute)
						})
						r.Get("/deployments/latest", deploy.GetLatest)
					})

					// --- Environments (project-scoped) ---
					r.Route(pathEnvironments, func(r chi.Router) {
						r.Get("/", env.List)
						r.Get("/{envName}", env.Get)

						// --- Env admins (env owner grants env-admin) ---
						// Listing needs read:project; add/remove auth is
						// in-handler (superuser, project admin, or env-admin).
						r.Route("/{envName}/admins", func(r chi.Router) {
							r.With(perm(db, "read", "project", param("projectName"), nil, nil)).
								Get("/", envadm.List)
							r.Post("/", envadm.Add)
							r.Delete("/{userID}", envadm.Remove)
						})
					})

					// --- Approval condition (project-scoped) ---
					// Roles are global (managed under /api/roles); the approval
					// condition references them by name and is editable by a
					// project admin (grant).
					r.With(perm(db, "grant", "", param("projectName"), nil, nil)).
						Put("/approval-condition", proj.UpdateApprovalCondition)

					// --- Project version history & manifest ---
					r.Route(pathVersions, func(r chi.Router) {
						r.With(perm(db, "read", "project", param("projectName"), nil, nil)).
							Get("/", proj.ListVersions)
						r.With(perm(db, "read", "project", param("projectName"), nil, nil)).
							Get(pathVersionID, proj.GetVersion)
					})
				})
			})

			// --- Global Values Groups ---
			r.Route("/global-values-groups", func(r chi.Router) {
				r.Get("/", gv.List)
				r.Post("/", gv.Create)

				r.Route("/{name}", func(r chi.Router) {
					r.With(perm(db, "read", "global_values", nil, nil, param("name"))).
						Get("/", gv.GetLatest)

					r.Route(pathVersions, func(r chi.Router) {
						r.With(perm(db, "read", "global_values", nil, nil, param("name"))).
							Get("/", gv.ListVersions)
						r.With(perm(db, "write", "global_values", nil, nil, param("name"))).
							Post("/", gv.AppendVersion)
						r.With(perm(db, "read", "global_values", nil, nil, param("name"))).
							Get(pathVersionID, gv.GetVersion)
					})

					// --- Approval condition (group-scoped) ---
					r.With(perm(db, "grant", "global_values", nil, nil, param("name"))).
						Put("/approval-condition", gv.UpdateApprovalCondition)
				})
			})

			// --- Pull Requests ---
			r.Route("/pull-requests", func(r chi.Router) {
				r.Post("/", pr.Create)
				r.Get("/", pr.List)
				r.Get("/{prID}", pr.Get)
				r.Post("/{prID}/close", pr.Close)
				r.Post("/{prID}/merge", pr.Merge)
				r.Post("/{prID}/approve", pr.Approve)
				r.Post("/{prID}/withdraw-approval", pr.WithdrawApproval)
				r.Post("/{prID}/submit", pr.SubmitDraft)
			})

			// --- Workspace (the project write surface) ---
			// The caller's single active Project PR. Every project mutation is
			// staged here and applied only on merge; reads overlay the base with
			// the caller's own staged changes.
			r.Route("/workspace/{projectName}", func(r chi.Router) {
				// Lifecycle
				r.Get("/", pr.GetActiveDraft)
				r.Get("/draft", pr.GetActiveDraft) // legacy alias
				r.Delete("/", pr.DiscardWorkspace)

				// Templates
				r.With(perm(db, "write", "project_templates", param("projectName"), nil, nil)).
					Post(pathTemplates, pr.StageTemplateCreate)
				r.With(perm(db, "write", "project_templates", param("projectName"), nil, nil)).
					Put(pathTemplateByName, pr.StageTemplateUpdate)
				r.With(perm(db, "delete", "project_templates", param("projectName"), nil, nil)).
					Delete(pathTemplateByName, pr.StageTemplateDelete)

				// Environments. Creating a *new* environment requires a wildcard
				// create:env_values(p, *) — only project admins, not env-admins
				// (whose create atom is scoped to a single existing env). Deleting
				// env X needs delete:project_values(p, X) — the admin's wildcard
				// matches, and an env-admin's env-scoped delete matches their env.
				r.With(perm(db, "create", "env_values", param("projectName"), middleware.Static("*"), nil)).
					Post(pathEnvironments, pr.StageEnvironmentCreate)
				r.With(perm(db, "delete", "project_values", param("projectName"), param("envName"), nil)).
					Delete("/environments/{envName}", pr.StageEnvironmentDelete)

				// Values (upsert gated by write; creating a brand-new set additionally
				// needs create:env_values, enforced in-handler)
				r.With(perm(db, "write", "project_values", param("projectName"), param("envName"), nil)).
					Put(pathEnvValues, pr.StageValuesUpsert)
				r.With(perm(db, "delete", "project_values", param("projectName"), param("envName"), nil)).
					Delete(pathEnvValues, pr.StageValuesDelete)

				// Overlay reads (base + caller's staged changes)
				r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
					Get(pathTemplates, pr.OverlayTemplates)
				r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
					Get(pathTemplateByName, pr.OverlayTemplate)
				r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
					Get(pathEnvironments, pr.OverlayEnvironments)
				r.With(perm(db, "read", "project_values", param("projectName"), param("envName"), nil)).
					Get(pathEnvValues, pr.OverlayValues)

				// Pre-submit validation (renders the overlay against every environment)
				r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
					Get("/validate", pr.ValidateWorkspace)

				// Change set / unstage
				r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
					Get("/changes", pr.ListChanges)
				r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
					Delete("/changes/{changeID}", pr.UnstageChange)
			})

			// --- Roles (global) ---
			// Roles are a single global namespace; their scope comes only from
			// their permission atoms. Listing is open to any authenticated user
			// (the approval-condition editor needs role names); all mutations are
			// superuser-only, enforced inside the handlers.
			r.Route("/roles", func(r chi.Router) {
				r.Get("/", role.List)
				r.Post("/", role.Create)

				r.Route("/{roleID}", func(r chi.Router) {
					r.Put("/permissions", role.EditPermissions)
					r.Delete("/", role.Delete)
					r.Post("/members", role.AssignUser)
					r.Delete("/members/{userID}", role.RemoveUser)
				})
			})

		})
	})

	return r
}
