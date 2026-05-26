package bddtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func mintToken(userID int64, username string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  float64(userID),
		"username": username,
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString(jwtSecret)
	Expect(err).NotTo(HaveOccurred())
	return signed
}

func seedUser(username, displayName string) int64 {
	var id int64
	err := testDB.QueryRowContext(context.Background(),
		`INSERT INTO users (username, display_name) VALUES ($1, $2) RETURNING id`,
		username, displayName,
	).Scan(&id)
	Expect(err).NotTo(HaveOccurred())
	return id
}

// seedSuperuser seeds a user with the superuser flag set. Superusers bypass all
// route-middleware and CheckPermission checks (including read:project).
func seedSuperuser(username, displayName string) int64 {
	var id int64
	err := testDB.QueryRowContext(context.Background(),
		`INSERT INTO users (username, display_name, superuser) VALUES ($1, $2, true) RETURNING id`,
		username, displayName,
	).Scan(&id)
	Expect(err).NotTo(HaveOccurred())
	return id
}

// addProjectMember adds targetUserID to the project via the members endpoint,
// acting as the given admin (who must hold grant on the project).
func addProjectMember(adminID int64, adminUsername, projectName string, targetUserID int64) {
	GinkgoHelper()
	rec := doRequest("POST", "/api/projects/"+projectName+"/members", map[string]any{
		"user_id": targetUserID,
	}, adminID, adminUsername)
	Expect(rec.Code).To(Equal(http.StatusCreated))
}

func seedSystemRole(userID int64) {
	var roleID int64
	err := testDB.QueryRowContext(context.Background(),
		`INSERT INTO roles (name, is_auto_created) VALUES ('system_admin', false) RETURNING id`,
	).Scan(&roleID)
	Expect(err).NotTo(HaveOccurred())

	_, err = testDB.ExecContext(context.Background(),
		`INSERT INTO role_permissions (role_id, action, resource) VALUES ($1, 'create', 'project')`,
		roleID,
	)
	Expect(err).NotTo(HaveOccurred())

	_, err = testDB.ExecContext(context.Background(),
		`INSERT INTO user_roles (user_id, role_id, granted_by) VALUES ($1, $2, $1)`,
		userID, roleID,
	)
	Expect(err).NotTo(HaveOccurred())
}

func seedGlobalValuesPermission(userID int64, gvName string) {
	var roleID int64
	roleName := fmt.Sprintf("gv_writer_%s_%d", gvName, userID)
	err := testDB.QueryRowContext(context.Background(),
		`INSERT INTO roles (name, is_auto_created) VALUES ($1, false) RETURNING id`,
		roleName,
	).Scan(&roleID)
	Expect(err).NotTo(HaveOccurred())

	_, err = testDB.ExecContext(context.Background(),
		`INSERT INTO role_permissions (role_id, action, resource, key_name) VALUES ($1, 'write', 'global_values', $2)`,
		roleID, gvName,
	)
	Expect(err).NotTo(HaveOccurred())

	_, err = testDB.ExecContext(context.Background(),
		`INSERT INTO user_roles (user_id, role_id, granted_by) VALUES ($1, $2, $1)`,
		userID, roleID,
	)
	Expect(err).NotTo(HaveOccurred())
}

func doRequest(method, path string, body any, userID int64, username string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != nil {
		b, err := json.Marshal(body)
		Expect(err).NotTo(HaveOccurred())
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+mintToken(userID, username))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decode[T any](rec *httptest.ResponseRecorder) T {
	var v T
	err := json.NewDecoder(rec.Body).Decode(&v)
	Expect(err).NotTo(HaveOccurred(), "failed to decode response: %s", rec.Body.String())
	return v
}

func truncateAll() {
	GinkgoHelper()
	_, err := testDB.ExecContext(context.Background(), `
		TRUNCATE users, environments, projects, project_config_templates,
		         project_config_values, global_values, roles, role_permissions,
		         user_roles, project_members, deployments, deployment_entries,
		         deployment_entry_global_refs, pull_requests, pr_changes,
		         pr_approvals CASCADE
	`)
	Expect(err).NotTo(HaveOccurred())
}

func createProject(userID int64, username, projectName string) map[string]any {
	rec := doRequest("POST", "/api/projects", map[string]any{
		"name": projectName,
	}, userID, username)
	Expect(rec.Code).To(Equal(http.StatusCreated))
	return decode[map[string]any](rec)
}

func createEnvironment(userID int64, username, projectName, envName string) map[string]any {
	// Environments are project-scoped and created via PR workflow.
	// For test convenience, insert directly into DB.
	var projectID int64
	err := testDB.QueryRowContext(context.Background(),
		`SELECT id FROM projects WHERE name = $1`, projectName).Scan(&projectID)
	Expect(err).NotTo(HaveOccurred())

	var env struct {
		ID        int64
		CreatedAt time.Time
	}
	err = testDB.QueryRowContext(context.Background(), `
		INSERT INTO environments (project_id, name, created_by)
		VALUES ($1, $2, $3) RETURNING id, created_at
	`, projectID, envName, userID).Scan(&env.ID, &env.CreatedAt)
	Expect(err).NotTo(HaveOccurred())

	return map[string]any{
		"id":         float64(env.ID),
		"project_id": float64(projectID),
		"name":       envName,
	}
}

// createTemplate seeds a published template (v1) directly in the DB. Authoring
// now goes through the workspace; this helper exists to set up live state for
// tests that exercise reads or downstream behaviour.
func createTemplate(userID int64, username, projectName, templateName, body string) map[string]any {
	var projectID int64
	err := testDB.QueryRowContext(context.Background(),
		`SELECT id FROM projects WHERE name = $1`, projectName).Scan(&projectID)
	Expect(err).NotTo(HaveOccurred())

	var id int64
	err = testDB.QueryRowContext(context.Background(), `
		INSERT INTO project_config_templates (project_id, template_name, version_id, body, created_by)
		VALUES ($1, $2, 1, $3, $4) RETURNING id
	`, projectID, templateName, body, userID).Scan(&id)
	Expect(err).NotTo(HaveOccurred())

	return map[string]any{
		"id":            float64(id),
		"project_id":    float64(projectID),
		"template_name": templateName,
		"version_id":    float64(1),
		"body":          body,
	}
}

// seedValues seeds a published value set version directly in the DB (computing
// the next version), returning the created version_id.
func seedValues(userID int64, projectName, envName string, payload map[string]any) int {
	var projectID, envID int64
	err := testDB.QueryRowContext(context.Background(),
		`SELECT id FROM projects WHERE name = $1`, projectName).Scan(&projectID)
	Expect(err).NotTo(HaveOccurred())
	err = testDB.QueryRowContext(context.Background(),
		`SELECT id FROM environments WHERE project_id = $1 AND name = $2`, projectID, envName).Scan(&envID)
	Expect(err).NotTo(HaveOccurred())

	raw, err := json.Marshal(payload)
	Expect(err).NotTo(HaveOccurred())

	var version int
	err = testDB.QueryRowContext(context.Background(), `
		INSERT INTO project_config_values (project_id, environment_id, version_id, payload, created_by)
		VALUES ($1, $2,
			COALESCE((SELECT version_id FROM project_config_values
			          WHERE project_id = $1 AND environment_id = $2
			          ORDER BY version_id DESC LIMIT 1), 0) + 1,
			$3, $4)
		RETURNING version_id
	`, projectID, envID, raw, userID).Scan(&version)
	Expect(err).NotTo(HaveOccurred())
	return version
}

// submitApproveMerge takes the caller's active workspace through submit →
// approve → merge so its staged changes become live state. The author is also
// the approver (works when the author satisfies the approval condition).
func submitApproveMerge(userID int64, username, projectName string) {
	GinkgoHelper()
	submitApproveMergeBy(userID, username, userID, username, projectName)
}

// submitApproveMergeBy is like submitApproveMerge but lets a different user
// (e.g. a project admin) supply the approval while the author merges.
func submitApproveMergeBy(authorID int64, authorName string, approverID int64, approverName, projectName string) {
	GinkgoHelper()
	rec := doRequest("GET", "/api/workspace/"+projectName, nil, authorID, authorName)
	Expect(rec.Code).To(Equal(http.StatusOK))
	prID := decode[map[string]any](rec)["id"].(float64)

	rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/submit", prID), map[string]any{"title": "test change"}, authorID, authorName)
	Expect(rec.Code).To(Equal(http.StatusOK))
	rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/approve", prID), nil, approverID, approverName)
	Expect(rec.Code).To(Equal(http.StatusOK))
	rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/merge", prID), nil, authorID, authorName)
	Expect(rec.Code).To(Equal(http.StatusOK))
}

func createGlobalValues(userID int64, username, name string, payload map[string]any) map[string]any {
	rec := doRequest("POST", "/api/global-values", map[string]any{
		"name":    name,
		"payload": payload,
	}, userID, username)
	Expect(rec.Code).To(Equal(http.StatusCreated))
	return decode[map[string]any](rec)
}

func seedTemplateWritePermission(userID int64, projectName string) {
	var roleID int64
	roleName := fmt.Sprintf("tmpl_writer_%s_%d", projectName, userID)
	err := testDB.QueryRowContext(context.Background(),
		`INSERT INTO roles (name, is_auto_created) VALUES ($1, false) RETURNING id`,
		roleName,
	).Scan(&roleID)
	Expect(err).NotTo(HaveOccurred())

	_, err = testDB.ExecContext(context.Background(),
		`INSERT INTO role_permissions (role_id, action, resource, key_project) VALUES ($1, 'write', 'project_templates', $2)`,
		roleID, projectName,
	)
	Expect(err).NotTo(HaveOccurred())

	_, err = testDB.ExecContext(context.Background(),
		`INSERT INTO user_roles (user_id, role_id, granted_by) VALUES ($1, $2, $1)`,
		userID, roleID,
	)
	Expect(err).NotTo(HaveOccurred())
}

// appendTemplateVersion seeds the next published version of a template directly
// in the DB, returning the new version_id.
func appendTemplateVersion(userID int64, username, projectName, templateName, body string) map[string]any {
	var projectID int64
	err := testDB.QueryRowContext(context.Background(),
		`SELECT id FROM projects WHERE name = $1`, projectName).Scan(&projectID)
	Expect(err).NotTo(HaveOccurred())

	var version int
	err = testDB.QueryRowContext(context.Background(), `
		INSERT INTO project_config_templates (project_id, template_name, version_id, body, created_by)
		VALUES ($1, $2,
			COALESCE((SELECT version_id FROM project_config_templates
			          WHERE project_id = $1 AND template_name = $2
			          ORDER BY version_id DESC LIMIT 1), 0) + 1,
			$3, $4)
		RETURNING version_id
	`, projectID, templateName, body, userID).Scan(&version)
	Expect(err).NotTo(HaveOccurred())

	return map[string]any{"version_id": float64(version), "body": body}
}
