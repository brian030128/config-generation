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

func (h *EnvironmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateEnvironmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required", "validation")
		return
	}

	var env models.Environment
	err := h.DB.QueryRowContext(r.Context(), `
		INSERT INTO environments (name, description) VALUES ($1, $2)
		RETURNING id, name, description, created_at
	`, req.Name, req.Description).Scan(&env.ID, &env.Name, &env.Description, &env.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "environment name already exists", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create environment", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, env)
}

func (h *EnvironmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	envID, err := urlParamInt64(r, "envID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid environment ID", "bad_request")
		return
	}

	var env models.Environment
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, name, description, created_at FROM environments WHERE id = $1
	`, envID).Scan(&env.ID, &env.Name, &env.Description, &env.CreatedAt)
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

func (h *EnvironmentHandler) GetByName(w http.ResponseWriter, r *http.Request) {
	envName := chi.URLParam(r, "envName")

	var env models.Environment
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT id, name, description, created_at FROM environments WHERE name = $1
	`, envName).Scan(&env.ID, &env.Name, &env.Description, &env.CreatedAt)
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

func (h *EnvironmentHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, name, description, created_at FROM environments ORDER BY name
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer rows.Close()

	var envs []models.Environment
	for rows.Next() {
		var e models.Environment
		if err := rows.Scan(&e.ID, &e.Name, &e.Description, &e.CreatedAt); err != nil {
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
