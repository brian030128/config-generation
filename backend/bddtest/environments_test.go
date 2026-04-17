package bddtest

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Environments", func() {
	var aliceID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
	})

	Context("creating an environment", func() {
		It("returns 201 with environment details", func() {
			rec := doRequest("POST", "/api/environments", map[string]any{
				"name":        "staging",
				"description": "Staging environment",
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["name"]).To(Equal("staging"))
			Expect(body["description"]).To(Equal("Staging environment"))
			Expect(body["id"]).NotTo(BeZero())
		})

		It("rejects duplicate environment names with 409", func() {
			createEnvironment(aliceID, "alice", "staging")

			rec := doRequest("POST", "/api/environments", map[string]any{
				"name": "staging",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusConflict))
		})

		It("rejects empty name with 400", func() {
			rec := doRequest("POST", "/api/environments", map[string]any{
				"name": "",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("listing environments", func() {
		It("returns all environments", func() {
			createEnvironment(aliceID, "alice", "staging")
			createEnvironment(aliceID, "alice", "prod")

			rec := doRequest("GET", "/api/environments", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(2))
		})

		It("returns empty list when none exist", func() {
			rec := doRequest("GET", "/api/environments", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(0))
		})
	})

	Context("getting an environment by ID", func() {
		It("returns 200 for existing environment", func() {
			env := createEnvironment(aliceID, "alice", "staging")
			envID := env["id"]

			rec := doRequest("GET", fmt.Sprintf("/api/environments/%v", envID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["name"]).To(Equal("staging"))
		})

		It("returns 404 for non-existent environment", func() {
			rec := doRequest("GET", "/api/environments/99999", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})
})
