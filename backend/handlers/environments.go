package handlers

import (
	"database/sql"
	"net/http"

	"github.com/brian/config-generation/backend/models"
	"github.com/go-chi/chi/v5"
)

type EnvironmentHandler struct {
	DB *sql.DB
}

func (h *EnvironmentHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, project_id, name, description, created_by, created_at
		FROM environments WHERE project_id = $1 ORDER BY name
	`, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer rows.Close()

	var envs []models.Environment
	for rows.Next() {
		var e models.Environment
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Name, &e.Description, &e.CreatedBy, &e.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		envs = append(envs, e)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	if envs == nil {
		envs = []models.Environment{}
	}
	writeJSON(w, http.StatusOK, models.ListResponse[models.Environment]{Items: envs, Count: len(envs)})
}

func (h *EnvironmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectID, err := resolveProjectID(r, h.DB)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	envName := chi.URLParam(r, "envName")

	var env models.Environment
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, project_id, name, description, created_by, created_at
		FROM environments WHERE project_id = $1 AND name = $2
	`, projectID, envName).Scan(&env.ID, &env.ProjectID, &env.Name, &env.Description, &env.CreatedBy, &env.CreatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "environment not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	writeJSON(w, http.StatusOK, env)
}
