package bddtest

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Projects", func() {
	var aliceID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		seedSystemRole(aliceID)
	})

	Context("creating a project", func() {
		It("returns 201 with project details and default approval condition", func() {
			rec := doRequest("POST", "/api/projects", map[string]any{
				"name": "billing-service",
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["name"]).To(Equal("billing-service"))
			Expect(body["created_by"]).To(BeEquivalentTo(aliceID))
			Expect(body["approval_condition"]).To(Equal("1 x project_admin"))
		})

		It("accepts a custom approval condition", func() {
			rec := doRequest("POST", "/api/projects", map[string]any{
				"name":               "billing-service",
				"approval_condition": "2 x project_developer",
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["approval_condition"]).To(Equal("2 x project_developer"))
		})

		It("auto-creates a project_admin role with correct permissions", func() {
			createProject(aliceID, "alice", "billing-service")

			rec := doRequest("GET", "/api/projects/billing-service/roles", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			items := body["items"].([]any)
			Expect(items).To(HaveLen(1))

			role := items[0].(map[string]any)
			Expect(role["name"]).To(Equal("project_admin:billing-service"))
			Expect(role["is_auto_created"]).To(BeTrue())

			perms := role["permissions"].([]any)
			Expect(perms).To(HaveLen(5))

			permSet := make(map[string]bool)
			for _, p := range perms {
				perm := p.(map[string]any)
				key := perm["action"].(string) + ":" + perm["resource"].(string)
				permSet[key] = true
			}
			Expect(permSet).To(HaveKey("write:project_templates"))
			Expect(permSet).To(HaveKey("create:env_values"))
			Expect(permSet).To(HaveKey("delete:project_values"))
			Expect(permSet).To(HaveKey("delete:project"))
			Expect(permSet).To(HaveKey("grant:"))
		})

		It("assigns the creator to the project_admin role", func() {
			createProject(aliceID, "alice", "billing-service")

			rec := doRequest("GET", "/api/projects/billing-service/roles", nil, aliceID, "alice")
			body := decode[map[string]any](rec)
			items := body["items"].([]any)
			role := items[0].(map[string]any)
			members := role["members"].([]any)
			Expect(members).To(HaveLen(1))
			member := members[0].(map[string]any)
			Expect(member["user_id"]).To(BeEquivalentTo(aliceID))
		})

		It("rejects duplicate project names with 409", func() {
			createProject(aliceID, "alice", "billing-service")

			rec := doRequest("POST", "/api/projects", map[string]any{
				"name": "billing-service",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusConflict))
		})

		It("rejects empty name with 400", func() {
			rec := doRequest("POST", "/api/projects", map[string]any{
				"name": "",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("listing projects", func() {
		It("returns all projects", func() {
			createProject(aliceID, "alice", "billing")
			createProject(aliceID, "alice", "payments")

			rec := doRequest("GET", "/api/projects", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(2))
		})

		It("returns empty list when none exist", func() {
			rec := doRequest("GET", "/api/projects", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(0))
		})
	})

	Context("getting a project by name", func() {
		It("returns 200 for existing project", func() {
			createProject(aliceID, "alice", "billing-service")

			rec := doRequest("GET", "/api/projects/billing-service", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["name"]).To(Equal("billing-service"))
		})

		It("returns 404 for non-existent project", func() {
			rec := doRequest("GET", "/api/projects/nonexistent", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	Context("deleting a project", func() {
		It("returns 204 when project admin deletes", func() {
			createProject(aliceID, "alice", "billing-service")

			rec := doRequest("DELETE", "/api/projects/billing-service", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNoContent))

			rec = doRequest("GET", "/api/projects/billing-service", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 403 without delete:project permission", func() {
			createProject(aliceID, "alice", "billing-service")

			bobID := seedUser("bob", "Bob Jones")
			rec := doRequest("DELETE", "/api/projects/billing-service", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})
	})
})
