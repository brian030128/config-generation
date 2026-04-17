package bddtest

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Project Config Values", func() {
	var (
		aliceID   int64
		stagingID float64
	)

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		seedSystemRole(aliceID)
		createProject(aliceID, "alice", "billing-service")
		env := createEnvironment(aliceID, "alice", "staging")
		stagingID = env["id"].(float64)
		createTemplate(aliceID, "alice", "billing-service", "app.yaml", "{{ .service_name }}")
	})

	Context("creating a value set v1", func() {
		It("returns 201 with version_id=1", func() {
			rec := doRequest("POST", "/api/projects/billing-service/values", map[string]any{
				"template_name":  "app.yaml",
				"environment_id": stagingID,
				"payload": map[string]any{
					"service_name": "billing",
					"env":          "staging",
				},
				"commit_message": "Initial values",
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(1))
			Expect(body["template_name"]).To(Equal("app.yaml"))
		})

		It("rejects duplicate (template, env) with 409", func() {
			doRequest("POST", "/api/projects/billing-service/values", map[string]any{
				"template_name":  "app.yaml",
				"environment_id": stagingID,
				"payload":        map[string]any{"key": "val"},
			}, aliceID, "alice")

			rec := doRequest("POST", "/api/projects/billing-service/values", map[string]any{
				"template_name":  "app.yaml",
				"environment_id": stagingID,
				"payload":        map[string]any{"key": "val2"},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusConflict))
		})
	})

	Context("appending value versions", func() {
		BeforeEach(func() {
			doRequest("POST", "/api/projects/billing-service/values", map[string]any{
				"template_name":  "app.yaml",
				"environment_id": stagingID,
				"payload":        map[string]any{"service_name": "billing-v1"},
			}, aliceID, "alice")
		})

		It("creates version 2 with updated payload", func() {
			rec := doRequest("POST", "/api/projects/billing-service/templates/app.yaml/envs/staging/values/versions", map[string]any{
				"payload": map[string]any{"service_name": "billing-v2"},
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(2))
		})

		It("preserves version 1 immutably", func() {
			doRequest("POST", "/api/projects/billing-service/templates/app.yaml/envs/staging/values/versions", map[string]any{
				"payload": map[string]any{"service_name": "billing-v2"},
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/projects/billing-service/templates/app.yaml/envs/staging/values/versions/1", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			payload := body["payload"].(map[string]any)
			Expect(payload["service_name"]).To(Equal("billing-v1"))
		})

		It("returns the latest version by default", func() {
			doRequest("POST", "/api/projects/billing-service/templates/app.yaml/envs/staging/values/versions", map[string]any{
				"payload": map[string]any{"service_name": "billing-v2"},
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/projects/billing-service/templates/app.yaml/envs/staging/values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(2))
		})
	})

	Context("values with global value references (config-generation spec section 5)", func() {
		BeforeEach(func() {
			createGlobalValues(aliceID, "alice", "test_db_values", map[string]any{
				"host":     "test-db.internal",
				"port":     5432,
				"username": "app",
				"password": "s3cret",
			})
		})

		It("stores ${global_values.key} reference strings verbatim", func() {
			rec := doRequest("POST", "/api/projects/billing-service/values", map[string]any{
				"template_name":  "app.yaml",
				"environment_id": stagingID,
				"payload": map[string]any{
					"service_name": "billing",
					"env":          "staging",
					"db_host":      "${test_db_values.host}",
					"db_port":      "${test_db_values.port}",
					"db_user":      "${test_db_values.username}",
					"db_password":  "${test_db_values.password}",
					"feature_flags": map[string]any{
						"new_checkout":    true,
						"legacy_invoices": false,
					},
				},
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))

			// Verify references are stored as-is (resolution happens at render time)
			rec = doRequest("GET", "/api/projects/billing-service/templates/app.yaml/envs/staging/values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			payload := body["payload"].(map[string]any)
			Expect(payload["db_host"]).To(Equal("${test_db_values.host}"))
			Expect(payload["db_port"]).To(Equal("${test_db_values.port}"))
			Expect(payload["db_user"]).To(Equal("${test_db_values.username}"))
			Expect(payload["db_password"]).To(Equal("${test_db_values.password}"))
			Expect(payload["service_name"]).To(Equal("billing"))
		})
	})

	Context("listing values for (project, env)", func() {
		It("returns the latest version of each value set", func() {
			createTemplate(aliceID, "alice", "billing-service", "database.conf", "{{ .db }}")

			doRequest("POST", "/api/projects/billing-service/values", map[string]any{
				"template_name":  "app.yaml",
				"environment_id": stagingID,
				"payload":        map[string]any{"service_name": "billing"},
			}, aliceID, "alice")
			doRequest("POST", "/api/projects/billing-service/values", map[string]any{
				"template_name":  "database.conf",
				"environment_id": stagingID,
				"payload":        map[string]any{"db": "postgres"},
			}, aliceID, "alice")

			rec := doRequest("GET", fmt.Sprintf("/api/projects/billing-service/envs/staging/values"), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(2))
		})
	})
})
