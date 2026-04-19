package bddtest

import (
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
		env := createEnvironment(aliceID, "alice", "billing-service", "staging")
		stagingID = env["id"].(float64)
		createTemplate(aliceID, "alice", "billing-service", "app.yaml", "{{ .service_name }}")
	})

	Context("creating a value set v1", func() {
		It("returns 201 with version_id=1", func() {
			rec := doRequest("POST", "/api/projects/billing-service/values", map[string]any{
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
		})

		It("rejects duplicate environment with 409", func() {
			doRequest("POST", "/api/projects/billing-service/values", map[string]any{
				"environment_id": stagingID,
				"payload":        map[string]any{"key": "val"},
			}, aliceID, "alice")

			rec := doRequest("POST", "/api/projects/billing-service/values", map[string]any{
				"environment_id": stagingID,
				"payload":        map[string]any{"key": "val2"},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusConflict))
		})
	})

	Context("appending value versions", func() {
		BeforeEach(func() {
			doRequest("POST", "/api/projects/billing-service/values", map[string]any{
				"environment_id": stagingID,
				"payload":        map[string]any{"service_name": "billing-v1"},
			}, aliceID, "alice")
		})

		It("creates version 2 with updated payload", func() {
			rec := doRequest("POST", "/api/projects/billing-service/envs/staging/values/versions", map[string]any{
				"payload": map[string]any{"service_name": "billing-v2"},
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(2))
		})

		It("preserves version 1 immutably", func() {
			doRequest("POST", "/api/projects/billing-service/envs/staging/values/versions", map[string]any{
				"payload": map[string]any{"service_name": "billing-v2"},
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/projects/billing-service/envs/staging/values/versions/1", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			payload := body["payload"].(map[string]any)
			Expect(payload["service_name"]).To(Equal("billing-v1"))
		})

		It("returns the latest version by default", func() {
			doRequest("POST", "/api/projects/billing-service/envs/staging/values/versions", map[string]any{
				"payload": map[string]any{"service_name": "billing-v2"},
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/projects/billing-service/envs/staging/values", nil, aliceID, "alice")
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
			rec = doRequest("GET", "/api/projects/billing-service/envs/staging/values", nil, aliceID, "alice")
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
})
