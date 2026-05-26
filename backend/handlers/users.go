package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/brian/config-generation/backend/models"
)

type UserHandler struct {
	DB *sql.DB
}

// Search returns up to 20 users matching the optional `search` query (matched
// case-insensitively against username and display name); with no query it
// returns the first 20 users by username. Any authenticated user may search the
// directory — this powers the project member picker. Sensitive columns
// (password hash, superuser flag) are never returned.
func (h *UserHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("search"))

	var (
		rows *sql.Rows
		err  error
	)
	if q == "" {
		rows, err = h.DB.QueryContext(r.Context(), `
			SELECT id, username, display_name, created_at
			FROM users
			ORDER BY username
			LIMIT 20
		`)
	} else {
		rows, err = h.DB.QueryContext(r.Context(), `
			SELECT id, username, display_name, created_at
			FROM users
			WHERE username ILIKE '%' || $1 || '%'
			   OR display_name ILIKE '%' || $1 || '%'
			ORDER BY username
			LIMIT 20
		`, q)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer rows.Close()

	usersList := []models.User{}
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		usersList = append(usersList, u)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.User]{Items: usersList, Count: len(usersList)})
}
