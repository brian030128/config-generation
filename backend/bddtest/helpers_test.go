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

func strPtr(s string) *string {
	return &s
}

func truncateAll() {
	GinkgoHelper()
	_, err := testDB.ExecContext(context.Background(), `
		TRUNCATE users, environments, projects, project_config_templates,
		         project_config_values, global_values, roles, role_permissions,
		         user_roles, deployments, deployment_entries,
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

func createEnvironment(userID int64, username, envName string) map[string]any {
	rec := doRequest("POST", "/api/environments", map[string]any{
		"name": envName,
	}, userID, username)
	Expect(rec.Code).To(Equal(http.StatusCreated))
	return decode[map[string]any](rec)
}

func createTemplate(userID int64, username, projectName, templateName, body string) map[string]any {
	rec := doRequest("POST", "/api/projects/"+projectName+"/templates", map[string]any{
		"template_name": templateName,
		"body":          body,
	}, userID, username)
	Expect(rec.Code).To(Equal(http.StatusCreated))
	return decode[map[string]any](rec)
}

func createGlobalValues(userID int64, username, name string, payload map[string]any) map[string]any {
	rec := doRequest("POST", "/api/global-values", map[string]any{
		"name":    name,
		"payload": payload,
	}, userID, username)
	Expect(rec.Code).To(Equal(http.StatusCreated))
	return decode[map[string]any](rec)
}
