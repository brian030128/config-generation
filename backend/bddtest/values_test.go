package bddtest

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Project Config Values", func() {
	var aliceID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		seedSystemRole(aliceID)
		createProject(aliceID, "alice", "billing-service")
		createEnvironment(aliceID, "alice", "billing-service", "staging")
		createTemplate(aliceID, "alice", "billing-service", "app.yaml", "{{ .service_name }}")
	})

	Context("creating and editing values through the workspace", func() {
		It("publishes a value set v1 on merge", func() {
			rec := doRequest("PUT", "/api/workspace/billing-service/envs/staging/values", map[string]any{
				"payload":        map[string]any{"service_name": "billing", "env": "staging"},
				"commit_message": "Initial values",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			submitApproveMerge(aliceID, "alice", "billing-service")

			rec = doRequest("GET", "/api/projects/billing-service/envs/staging/values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["version_id"]).To(BeEquivalentTo(1))
		})

		It("publishes v2 when editing an existing value set", func() {
			seedValues(aliceID, "billing-service", "staging", map[string]any{"service_name": "billing-v1"})

			rec := doRequest("PUT", "/api/workspace/billing-service/envs/staging/values", map[string]any{
				"payload": map[string]any{"service_name": "billing-v2"},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			submitApproveMerge(aliceID, "alice", "billing-service")

			rec = doRequest("GET", "/api/projects/billing-service/envs/staging/values", nil, aliceID, "alice")
			body := decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(2))
			Expect(body["payload"].(map[string]any)["service_name"]).To(Equal("billing-v2"))
		})
	})

	Context("full-copy versioning (read model)", func() {
		BeforeEach(func() {
			seedValues(aliceID, "billing-service", "staging", map[string]any{"service_name": "billing-v1"})
		})

		It("preserves version 1 immutably after a new version", func() {
			seedValues(aliceID, "billing-service", "staging", map[string]any{"service_name": "billing-v2"})

			rec := doRequest("GET", "/api/projects/billing-service/envs/staging/values/versions/1", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["payload"].(map[string]any)["service_name"]).To(Equal("billing-v1"))
		})

		It("returns the latest version by default", func() {
			seedValues(aliceID, "billing-service", "staging", map[string]any{"service_name": "billing-v2"})

			rec := doRequest("GET", "/api/projects/billing-service/envs/staging/values", nil, aliceID, "alice")
			Expect(decode[map[string]any](rec)["version_id"]).To(BeEquivalentTo(2))
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

		It("stores ${global_values.key} reference strings verbatim through a merge", func() {
			rec := doRequest("PUT", "/api/workspace/billing-service/envs/staging/values", map[string]any{
				"payload": map[string]any{
					"service_name": "billing",
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
			Expect(rec.Code).To(Equal(http.StatusOK))

			submitApproveMerge(aliceID, "alice", "billing-service")

			rec = doRequest("GET", "/api/projects/billing-service/envs/staging/values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			payload := decode[map[string]any](rec)["payload"].(map[string]any)
			Expect(payload["db_host"]).To(Equal("${test_db_values.host}"))
			Expect(payload["db_port"]).To(Equal("${test_db_values.port}"))
			Expect(payload["db_user"]).To(Equal("${test_db_values.username}"))
			Expect(payload["db_password"]).To(Equal("${test_db_values.password}"))
			Expect(payload["service_name"]).To(Equal("billing"))
		})
	})
})
