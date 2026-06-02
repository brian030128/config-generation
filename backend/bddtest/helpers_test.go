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
		TRUNCATE users, environments, projects,
		         project_versions, project_version_templates, project_version_values,
		         project_config_templates, project_config_values,
		         global_values, global_values_groups, global_values_group_versions,
		         global_values_group_version_entries,
		         roles, role_permissions, user_roles, project_members,
		         deployments, deployment_entries, deployment_group_refs,
		         pull_requests, pr_changes, pr_approvals CASCADE
	`)
	Expect(err).NotTo(HaveOccurred())
}

// appendProjectSnapshot creates a new project_version row that copies forward
// the project's previous version's links, then applies the supplied template
// and values overrides. Used by the seed helpers to keep the snapshot layer in
// sync with directly-inserted content rows.
func appendProjectSnapshot(projectID int64, userID int64, templateLinks map[string]int64, valueLinks map[int64]int64) int64 {
	GinkgoHelper()
	ctx := context.Background()
	var prevID *int64
	var prevOrd *int
	_ = testDB.QueryRowContext(ctx, `
		SELECT id, version_id FROM project_versions
		WHERE project_id = $1 ORDER BY version_id DESC LIMIT 1
	`, projectID).Scan(&prevID, &prevOrd)
	nextOrd := 1
	if prevOrd != nil {
		nextOrd = *prevOrd + 1
	}
	var newID int64
	err := testDB.QueryRowContext(ctx, `
		INSERT INTO project_versions (project_id, version_id, parent_version_id, created_by)
		VALUES ($1, $2, $3, $4) RETURNING id
	`, projectID, nextOrd, prevID, userID).Scan(&newID)
	Expect(err).NotTo(HaveOccurred())
	if prevID != nil {
		_, err = testDB.ExecContext(ctx, `
			INSERT INTO project_version_templates (project_version_id, template_name, template_row_id)
			SELECT $1, template_name, template_row_id FROM project_version_templates WHERE project_version_id = $2
		`, newID, *prevID)
		Expect(err).NotTo(HaveOccurred())
		_, err = testDB.ExecContext(ctx, `
			INSERT INTO project_version_values (project_version_id, environment_id, values_row_id)
			SELECT $1, environment_id, values_row_id FROM project_version_values WHERE project_version_id = $2
		`, newID, *prevID)
		Expect(err).NotTo(HaveOccurred())
	}
	for name, rowID := range templateLinks {
		_, err = testDB.ExecContext(ctx, `
			INSERT INTO project_version_templates (project_version_id, template_name, template_row_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (project_version_id, template_name) DO UPDATE SET template_row_id = EXCLUDED.template_row_id
		`, newID, name, rowID)
		Expect(err).NotTo(HaveOccurred())
	}
	for envID, rowID := range valueLinks {
		_, err = testDB.ExecContext(ctx, `
			INSERT INTO project_version_values (project_version_id, environment_id, values_row_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (project_version_id, environment_id) DO UPDATE SET values_row_id = EXCLUDED.values_row_id
		`, newID, envID, rowID)
		Expect(err).NotTo(HaveOccurred())
	}
	return newID
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

// createTemplate seeds a published template directly in the DB plus the
// project_version that snapshots it. Authoring now goes through the workspace;
// this helper exists to set up live state for tests that exercise reads or
// downstream behaviour.
func createTemplate(userID int64, username, projectName, templateName, body string) map[string]any {
	var projectID int64
	err := testDB.QueryRowContext(context.Background(),
		`SELECT id FROM projects WHERE name = $1`, projectName).Scan(&projectID)
	Expect(err).NotTo(HaveOccurred())

	var id int64
	err = testDB.QueryRowContext(context.Background(), `
		INSERT INTO project_config_templates (project_id, template_name, version_id, body, created_by)
		VALUES ($1, $2,
		        COALESCE((SELECT MAX(version_id) FROM project_config_templates
		                  WHERE project_id = $1 AND template_name = $2), 0) + 1,
		        $3, $4) RETURNING id
	`, projectID, templateName, body, userID).Scan(&id)
	Expect(err).NotTo(HaveOccurred())

	pvID := appendProjectSnapshot(projectID, userID,
		map[string]int64{templateName: id}, nil)
	var pvOrd int
	_ = testDB.QueryRowContext(context.Background(),
		`SELECT version_id FROM project_versions WHERE id = $1`, pvID).Scan(&pvOrd)

	return map[string]any{
		"id":            float64(id),
		"project_id":    float64(projectID),
		"template_name": templateName,
		"version_id":    float64(pvOrd),
		"body":          body,
	}
}

// seedValues seeds a published value set version directly in the DB plus the
// project_version that snapshots it. Returns the (per-project) version_id of
// the resulting project_version.
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

	var rowID int64
	err = testDB.QueryRowContext(context.Background(), `
		INSERT INTO project_config_values (project_id, environment_id, version_id, payload, created_by)
		VALUES ($1, $2,
			COALESCE((SELECT MAX(version_id) FROM project_config_values
			          WHERE project_id = $1 AND environment_id = $2), 0) + 1,
			$3, $4)
		RETURNING id
	`, projectID, envID, raw, userID).Scan(&rowID)
	Expect(err).NotTo(HaveOccurred())

	pvID := appendProjectSnapshot(projectID, userID, nil, map[int64]int64{envID: rowID})
	var pvOrd int
	_ = testDB.QueryRowContext(context.Background(),
		`SELECT version_id FROM project_versions WHERE id = $1`, pvID).Scan(&pvOrd)
	return pvOrd
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

// seedGlobalValues seeds a singleton global values group named `name` with one
// entry of the same name carrying `payload`. The flat map ${name.key} that the
// renderer expects is built by merging every entry's payload at lookup time, so
// putting the values under a single entry keeps the test ergonomics from the
// old per-name model.
func seedGlobalValues(userID int64, name string, payload map[string]any) {
	GinkgoHelper()
	raw, err := json.Marshal(payload)
	Expect(err).NotTo(HaveOccurred())
	ctx := context.Background()

	var groupID int64
	err = testDB.QueryRowContext(ctx, `
		INSERT INTO global_values_groups (name, approval_condition, created_by)
		VALUES ($1, $2, $3)
		RETURNING id
	`, name, "1 x "+name+"_gv_group_admin", userID).Scan(&groupID)
	Expect(err).NotTo(HaveOccurred())

	var ggvID int64
	err = testDB.QueryRowContext(ctx, `
		INSERT INTO global_values_group_versions (group_id, version_id, created_by)
		VALUES ($1, 1, $2) RETURNING id
	`, groupID, userID).Scan(&ggvID)
	Expect(err).NotTo(HaveOccurred())

	var rowID int64
	err = testDB.QueryRowContext(ctx, `
		INSERT INTO global_values (group_id, name, payload, created_by)
		VALUES ($1, $2, $3, $4) RETURNING id
	`, groupID, name, raw, userID).Scan(&rowID)
	Expect(err).NotTo(HaveOccurred())

	_, err = testDB.ExecContext(ctx, `
		INSERT INTO global_values_group_version_entries (group_version_id, gv_name, gv_row_id)
		VALUES ($1, $2, $3)
	`, ggvID, name, rowID)
	Expect(err).NotTo(HaveOccurred())
}

// createGlobalValues creates a new global-values group via the API. The values
// map defaults to one entry named after the group, carrying the full payload —
// matching the legacy "one name per entry" call shape callers were using.
func createGlobalValues(userID int64, username, name string, payload map[string]any) map[string]any {
	rec := doRequest("POST", "/api/global-values-groups", map[string]any{
		"name": name,
		"values": map[string]any{
			name: payload,
		},
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

// appendTemplateVersion seeds the next content row of a template plus a new
// project_version snapshotting it. Returns the per-project version_id of the
// new project_version under the `version_id` key (keeping the same shape the
// old per-template call returned).
func appendTemplateVersion(userID int64, username, projectName, templateName, body string) map[string]any {
	var projectID int64
	err := testDB.QueryRowContext(context.Background(),
		`SELECT id FROM projects WHERE name = $1`, projectName).Scan(&projectID)
	Expect(err).NotTo(HaveOccurred())

	var rowID int64
	err = testDB.QueryRowContext(context.Background(), `
		INSERT INTO project_config_templates (project_id, template_name, version_id, body, created_by)
		VALUES ($1, $2,
			COALESCE((SELECT MAX(version_id) FROM project_config_templates
			          WHERE project_id = $1 AND template_name = $2), 0) + 1,
			$3, $4)
		RETURNING id
	`, projectID, templateName, body, userID).Scan(&rowID)
	Expect(err).NotTo(HaveOccurred())

	pvID := appendProjectSnapshot(projectID, userID, map[string]int64{templateName: rowID}, nil)
	var pvOrd int
	_ = testDB.QueryRowContext(context.Background(),
		`SELECT version_id FROM project_versions WHERE id = $1`, pvID).Scan(&pvOrd)

	return map[string]any{"version_id": float64(pvOrd), "body": body}
}
