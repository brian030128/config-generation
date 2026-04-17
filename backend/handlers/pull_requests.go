package handlers

import (
	"database/sql"
	"net/http"

	"github.com/brian/config-generation/backend/models"
)

type PullRequestHandler struct {
	DB *sql.DB
}

func (h *PullRequestHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	var req models.CreatePullRequestRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required", "validation")
		return
	}
	if req.ObjectType != "global_values" {
		writeError(w, http.StatusBadRequest, "unsupported object_type; only global_values is supported", "validation")
		return
	}
	if req.GlobalValuesName == nil || *req.GlobalValuesName == "" {
		writeError(w, http.StatusBadRequest, "global_values_name is required for global_values changes", "validation")
		return
	}
	if req.ProposedPayload == "" {
		writeError(w, http.StatusBadRequest, "proposed_payload is required", "validation")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer tx.Rollback()

	// Look up the current latest version_id for the global values entry.
	var baseVersionID int
	err = tx.QueryRowContext(r.Context(), `
		SELECT COALESCE(
			(SELECT version_id FROM global_values
			 WHERE name = $1
			 ORDER BY version_id DESC LIMIT 1),
		0)
	`, *req.GlobalValuesName).Scan(&baseVersionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	if baseVersionID == 0 {
		writeError(w, http.StatusNotFound, "global values entry not found", "not_found")
		return
	}

	// Create the pull request row.
	var pr models.PullRequest
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO pull_requests (project_id, author_id, title, description, status)
		VALUES (NULL, $1, $2, $3, 'open')
		RETURNING id, project_id, author_id, title, description, status,
		          is_conflicted, created_at, updated_at, merged_at, closed_at
	`, user.UserID, req.Title, req.Description).Scan(
		&pr.ID, &pr.ProjectID, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create pull request", "internal")
		return
	}

	// Create the pr_changes row.
	var change models.PRChange
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO pr_changes (pr_id, object_type, global_values_name, base_version_id, proposed_payload)
		VALUES ($1, 'global_values', $2, $3, $4)
		RETURNING id, pr_id, object_type, project_id, template_name,
		          environment_id, global_values_name, base_version_id,
		          proposed_payload, created_at
	`, pr.ID, *req.GlobalValuesName, baseVersionID, req.ProposedPayload).Scan(
		&change.ID, &change.PRID, &change.ObjectType, &change.ProjectID,
		&change.TemplateName, &change.EnvironmentID, &change.GlobalValuesName,
		&change.BaseVersionID, &change.ProposedPayload, &change.CreatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create pr change", "internal")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit", "internal")
		return
	}

	pr.Changes = []models.PRChange{change}
	writeJSON(w, http.StatusCreated, pr)
}

func (h *PullRequestHandler) Get(w http.ResponseWriter, r *http.Request) {
	prID, err := urlParamInt64(r, "prID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PR ID", "bad_request")
		return
	}

	var pr models.PullRequest
	err = h.DB.QueryRowContext(r.Context(), `
		SELECT id, project_id, author_id, title, description, status,
		       is_conflicted, created_at, updated_at, merged_at, closed_at
		FROM pull_requests WHERE id = $1
	`, prID).Scan(
		&pr.ID, &pr.ProjectID, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
	)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "pull request not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, pr_id, object_type, project_id, template_name,
		       environment_id, global_values_name, base_version_id,
		       proposed_payload, created_at
		FROM pr_changes WHERE pr_id = $1
		ORDER BY id
	`, prID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer rows.Close()

	pr.Changes = []models.PRChange{}
	for rows.Next() {
		var c models.PRChange
		if err := rows.Scan(
			&c.ID, &c.PRID, &c.ObjectType, &c.ProjectID, &c.TemplateName,
			&c.EnvironmentID, &c.GlobalValuesName, &c.BaseVersionID,
			&c.ProposedPayload, &c.CreatedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		pr.Changes = append(pr.Changes, c)
	}

	writeJSON(w, http.StatusOK, pr)
}

func (h *PullRequestHandler) Close(w http.ResponseWriter, r *http.Request) {
	prID, err := urlParamInt64(r, "prID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PR ID", "bad_request")
		return
	}

	var currentStatus string
	err = h.DB.QueryRowContext(r.Context(),
		`SELECT status FROM pull_requests WHERE id = $1`, prID,
	).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "pull request not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}

	if currentStatus != "draft" && currentStatus != "open" && currentStatus != "approved" {
		writeError(w, http.StatusConflict, "pull request cannot be closed in its current state", "conflict")
		return
	}

	var pr models.PullRequest
	err = h.DB.QueryRowContext(r.Context(), `
		UPDATE pull_requests
		SET status = 'closed', closed_at = NOW(), updated_at = NOW()
		WHERE id = $1
		RETURNING id, project_id, author_id, title, description, status,
		          is_conflicted, created_at, updated_at, merged_at, closed_at
	`, prID).Scan(
		&pr.ID, &pr.ProjectID, &pr.AuthorID, &pr.Title, &pr.Description,
		&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
		&pr.MergedAt, &pr.ClosedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to close pull request", "internal")
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

func (h *PullRequestHandler) List(w http.ResponseWriter, r *http.Request) {
	gvName := r.URL.Query().Get("global_values_name")

	var (
		rows *sql.Rows
		err  error
	)

	if gvName != "" {
		rows, err = h.DB.QueryContext(r.Context(), `
			SELECT DISTINCT p.id, p.project_id, p.author_id, p.title, p.description,
			       p.status, p.is_conflicted, p.created_at, p.updated_at,
			       p.merged_at, p.closed_at
			FROM pull_requests p
			JOIN pr_changes c ON c.pr_id = p.id
			WHERE c.object_type = 'global_values' AND c.global_values_name = $1
			ORDER BY p.created_at DESC
		`, gvName)
	} else {
		rows, err = h.DB.QueryContext(r.Context(), `
			SELECT id, project_id, author_id, title, description, status,
			       is_conflicted, created_at, updated_at, merged_at, closed_at
			FROM pull_requests
			ORDER BY created_at DESC
		`)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "internal")
		return
	}
	defer rows.Close()

	items := []models.PullRequest{}
	for rows.Next() {
		var pr models.PullRequest
		if err := rows.Scan(
			&pr.ID, &pr.ProjectID, &pr.AuthorID, &pr.Title, &pr.Description,
			&pr.Status, &pr.IsConflicted, &pr.CreatedAt, &pr.UpdatedAt,
			&pr.MergedAt, &pr.ClosedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "database error", "internal")
			return
		}
		items = append(items, pr)
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.PullRequest]{Items: items, Count: len(items)})
}
