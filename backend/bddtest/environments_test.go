package bddtest

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Environments (project-scoped)", func() {
	var aliceID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		seedSystemRole(aliceID)
		createProject(aliceID, "alice", "billing-service")
	})

	Context("listing environments for a project", func() {
		It("returns project-scoped environments", func() {
			createEnvironment(aliceID, "alice", "billing-service", "staging")
			createEnvironment(aliceID, "alice", "billing-service", "prod")

			rec := doRequest("GET", "/api/projects/billing-service/environments", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(2))
		})

		It("returns empty list when none exist", func() {
			rec := doRequest("GET", "/api/projects/billing-service/environments", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(0))
		})

		It("does not return environments from other projects", func() {
			seedExtraSystemRole(aliceID)
			createProject(aliceID, "alice", "payments-service")
			createEnvironment(aliceID, "alice", "billing-service", "staging")
			createEnvironment(aliceID, "alice", "payments-service", "staging")

			rec := doRequest("GET", "/api/projects/billing-service/environments", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(1))
		})
	})

	Context("getting an environment by name", func() {
		It("returns 200 for existing environment", func() {
			createEnvironment(aliceID, "alice", "billing-service", "staging")

			rec := doRequest("GET", "/api/projects/billing-service/environments/staging", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["name"]).To(Equal("staging"))
		})

		It("returns 404 for non-existent environment", func() {
			rec := doRequest("GET", "/api/projects/billing-service/environments/nonexistent", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	Context("staging environment creation via draft PR", func() {
		It("stages an environment change in the draft", func() {
			rec := doRequest("POST", "/api/workspace/billing-service/stage", map[string]any{
				"object_type":      "environment",
				"proposed_payload": `{"name":"staging"}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(1))
			change := changes[0].(map[string]any)
			Expect(change["object_type"]).To(Equal("environment"))
		})
	})
})
