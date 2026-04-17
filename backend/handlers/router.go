package handlers

import (
	"database/sql"

	"github.com/brian/config-generation/backend/middleware"
	"github.com/go-chi/chi/v5"
)

func NewRouter(db *sql.DB, jwtSecret []byte) chi.Router {
	r := chi.NewRouter()

	// Global middleware: all routes require JWT auth.
	r.Use(middleware.JWTAuth(jwtSecret))

	proj := &ProjectHandler{DB: db}
	env := &EnvironmentHandler{DB: db}
	tmpl := &TemplateHandler{DB: db}
	vals := &ValuesHandler{DB: db}
	gv := &GlobalValuesHandler{DB: db}
	role := &RoleHandler{DB: db}

	perm := middleware.RequirePermission
	param := middleware.URLParam

	r.Route("/api", func(r chi.Router) {

		// --- Projects ---
		r.Route("/projects", func(r chi.Router) {
			r.With(perm(db, "create", "project", nil, nil, nil)).
				Post("/", proj.Create)
			r.Get("/", proj.List)

			r.Route("/{projectName}", func(r chi.Router) {
				r.Get("/", proj.Get)
				r.With(perm(db, "delete", "project", param("projectName"), nil, nil)).
					Delete("/", proj.Delete)

				// --- Templates ---
				r.Route("/templates", func(r chi.Router) {
					r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
						Get("/", tmpl.ListForProject)
					r.With(perm(db, "write", "project_templates", param("projectName"), nil, nil)).
						Post("/", tmpl.Create)

					r.Route("/{templateName}", func(r chi.Router) {
						r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
							Get("/", tmpl.GetLatest)

						r.Route("/versions", func(r chi.Router) {
							r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
								Get("/", tmpl.ListVersions)
							r.With(perm(db, "write", "project_templates", param("projectName"), nil, nil)).
								Post("/", tmpl.AppendVersion)
							r.With(perm(db, "read", "project_templates", param("projectName"), nil, nil)).
								Get("/{versionID}", tmpl.GetVersion)
						})

						// --- Values per (template, env) ---
						r.Route("/envs/{envName}/values", func(r chi.Router) {
							r.With(perm(db, "read", "project_values", param("projectName"), param("envName"), nil)).
								Get("/", vals.GetLatest)

							r.Route("/versions", func(r chi.Router) {
								r.With(perm(db, "write", "project_values", param("projectName"), param("envName"), nil)).
									Post("/", vals.AppendVersion)
								r.With(perm(db, "read", "project_values", param("projectName"), param("envName"), nil)).
									Get("/{versionID}", vals.GetVersion)
							})
						})
					})
				})

				// --- Create new value set (v1) ---
				r.With(perm(db, "create", "env_values", param("projectName"), nil, nil)).
					Post("/values", vals.Create)

				// --- List values for (project, env) ---
				r.Route("/envs/{envName}/values", func(r chi.Router) {
					r.With(perm(db, "read", "project_values", param("projectName"), param("envName"), nil)).
						Get("/", vals.ListForProjectEnv)
				})

				// --- Roles (project-scoped) ---
				r.Route("/roles", func(r chi.Router) {
					r.With(perm(db, "grant", "", param("projectName"), nil, nil)).
						Post("/", role.Create)
					r.With(perm(db, "grant", "", param("projectName"), nil, nil)).
						Get("/", role.ListForProject)
				})
			})
		})

		// --- Environments ---
		r.Route("/environments", func(r chi.Router) {
			r.Get("/", env.List)
			r.Post("/", env.Create)
			r.Get("/{envID}", env.Get)
		})

		// --- Global Values ---
		r.Route("/global-values", func(r chi.Router) {
			r.Get("/", gv.List)
			r.Post("/", gv.Create)

			r.Route("/{name}", func(r chi.Router) {
				r.With(perm(db, "read", "global_values", nil, nil, param("name"))).
					Get("/", gv.GetLatest)

				r.Route("/versions", func(r chi.Router) {
					r.With(perm(db, "read", "global_values", nil, nil, param("name"))).
						Get("/", gv.ListVersions)
					r.With(perm(db, "write", "global_values", nil, nil, param("name"))).
						Post("/", gv.AppendVersion)
					r.With(perm(db, "read", "global_values", nil, nil, param("name"))).
						Get("/{versionID}", gv.GetVersion)
				})
			})
		})

		// --- Roles (non-project-scoped operations by role ID) ---
		// Permission checks happen inside handlers since we need to look up the role's project first.
		r.Route("/roles/{roleID}", func(r chi.Router) {
			r.Put("/permissions", role.EditPermissions)
			r.Delete("/", role.Delete)
			r.Post("/members", role.AssignUser)
			r.Delete("/members/{userID}", role.RemoveUser)
		})
	})

	return r
}
