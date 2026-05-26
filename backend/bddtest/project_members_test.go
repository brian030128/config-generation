package bddtest

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// projectNamesInList returns the set of project names in a GET /api/projects body.
func projectNamesInList(rec *httptest.ResponseRecorder) map[string]bool {
	GinkgoHelper()
	body := decode[map[string]any](rec)
	names := map[string]bool{}
	for _, item := range body["items"].([]any) {
		names[item.(map[string]any)["name"].(string)] = true
	}
	return names
}

var _ = Describe("Project Members (read:project)", func() {
	var (
		aliceID int64
		bobID   int64
	)

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		bobID = seedUser("bob", "Bob Jones")
		seedSystemRole(aliceID)
		createProject(aliceID, "alice", "billing")
	})

	Context("read:project gate on GET /api/projects/{p}", func() {
		It("allows the creator, who is an auto-member", func() {
			rec := doRequest("GET", "/api/projects/billing", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		It("denies a user with no membership or permissions", func() {
			rec := doRequest("GET", "/api/projects/billing", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("denies a non-member even if they can read the project's templates", func() {
			// read:project is not implied by any other project permission — it
			// comes only from membership. A template reader who is not a member
			// can read templates but not the project shell.
			createTemplate(aliceID, "alice", "billing", "app.yaml", "body")
			roleID := createCustomRole(aliceID, "alice", "billing", "billing-tmpl-reader", []map[string]any{
				{"action": "read", "resource": "project_templates", "key_project": "billing"},
			})
			assignUserToRole(aliceID, "alice", roleID, bobID)

			rec := doRequest("GET", "/api/projects/billing", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))

			rec = doRequest("GET", "/api/projects/billing/templates", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		It("allows a superuser to read any project", func() {
			rootID := seedSuperuser("root", "Root")
			rec := doRequest("GET", "/api/projects/billing", nil, rootID, "root")
			Expect(rec.Code).To(Equal(http.StatusOK))
		})
	})

	Context("a project admin adding a member", func() {
		It("grants the member read:project and lists the project for them", func() {
			addProjectMember(aliceID, "alice", "billing", bobID)

			rec := doRequest("GET", "/api/projects/billing", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))

			rec = doRequest("GET", "/api/projects", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(projectNamesInList(rec)).To(HaveKey("billing"))
		})

		It("does not grant the member any template or values reads", func() {
			createTemplate(aliceID, "alice", "billing", "app.yaml", "body")
			createEnvironment(aliceID, "alice", "billing", "staging")
			seedValues(aliceID, "billing", "staging", map[string]any{"k": "v"})

			addProjectMember(aliceID, "alice", "billing", bobID)

			By("templates are gated by read:project_templates")
			rec := doRequest("GET", "/api/projects/billing/templates", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))

			By("env values are gated by read:project_values")
			rec = doRequest("GET", "/api/projects/billing/envs/staging/values", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("lists the project's members and reports viewer_can_manage for an admin", func() {
			addProjectMember(aliceID, "alice", "billing", bobID)

			rec := doRequest("GET", "/api/projects/billing/members", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(2)) // creator + bob
			Expect(body["viewer_can_manage"]).To(BeTrue())
		})

		It("reports viewer_can_manage=false for a plain member who can still see the list", func() {
			addProjectMember(aliceID, "alice", "billing", bobID)

			rec := doRequest("GET", "/api/projects/billing/members", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(2)) // bob can read the member list
			Expect(body["viewer_can_manage"]).To(BeFalse())
		})

		It("returns 409 when the user is already a member", func() {
			addProjectMember(aliceID, "alice", "billing", bobID)
			rec := doRequest("POST", "/api/projects/billing/members", map[string]any{
				"user_id": bobID,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusConflict))
		})

		It("returns 404 when the target user does not exist", func() {
			rec := doRequest("POST", "/api/projects/billing/members", map[string]any{
				"user_id": 999999,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("denies a non-admin (no grant) from adding members", func() {
			addProjectMember(aliceID, "alice", "billing", bobID) // bob is a plain member
			carolID := seedUser("carol", "Carol Davis")

			rec := doRequest("POST", "/api/projects/billing/members", map[string]any{
				"user_id": carolID,
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})
	})

	Context("a project admin removing a member", func() {
		It("revokes read:project and removes the project from their list", func() {
			addProjectMember(aliceID, "alice", "billing", bobID)

			rec := doRequest("DELETE", fmt.Sprintf("/api/projects/billing/members/%d", bobID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNoContent))

			rec = doRequest("GET", "/api/projects/billing", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))

			rec = doRequest("GET", "/api/projects", nil, bobID, "bob")
			Expect(projectNamesInList(rec)).NotTo(HaveKey("billing"))
		})

		It("also revokes the member's project-scoped role assignments", func() {
			createTemplate(aliceID, "alice", "billing", "app.yaml", "body")
			roleID := createCustomRole(aliceID, "alice", "billing", "billing-tmpl-reader", []map[string]any{
				{"action": "read", "resource": "project_templates", "key_project": "billing"},
			})
			assignUserToRole(aliceID, "alice", roleID, bobID)
			addProjectMember(aliceID, "alice", "billing", bobID)

			By("bob can read templates while a member")
			rec := doRequest("GET", "/api/projects/billing/templates", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))

			By("removing membership revokes the template-reader role too")
			rec = doRequest("DELETE", fmt.Sprintf("/api/projects/billing/members/%d", bobID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNoContent))

			rec = doRequest("GET", "/api/projects/billing/templates", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("refuses to remove the sole project admin", func() {
			rec := doRequest("DELETE", fmt.Sprintf("/api/projects/billing/members/%d", aliceID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 404 when the target is not a member", func() {
			carolID := seedUser("carol", "Carol Davis")
			rec := doRequest("DELETE", fmt.Sprintf("/api/projects/billing/members/%d", carolID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("denies a non-admin (no grant) from removing members", func() {
			addProjectMember(aliceID, "alice", "billing", bobID)
			rec := doRequest("DELETE", fmt.Sprintf("/api/projects/billing/members/%d", bobID), nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})
	})

	Context("GET /api/projects list scoping", func() {
		It("shows each user only the projects they are a member of", func() {
			carolID := seedUser("carol", "Carol Davis")
			seedExtraSystemRole(carolID)
			createProject(carolID, "carol", "payments")

			By("alice sees only billing")
			rec := doRequest("GET", "/api/projects", nil, aliceID, "alice")
			names := projectNamesInList(rec)
			Expect(names).To(HaveKey("billing"))
			Expect(names).NotTo(HaveKey("payments"))

			By("carol sees only payments")
			rec = doRequest("GET", "/api/projects", nil, carolID, "carol")
			names = projectNamesInList(rec)
			Expect(names).To(HaveKey("payments"))
			Expect(names).NotTo(HaveKey("billing"))
		})

		It("returns an empty list for a user who is a member of nothing", func() {
			rec := doRequest("GET", "/api/projects", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(0))
		})

		It("shows a superuser every project", func() {
			carolID := seedUser("carol", "Carol Davis")
			seedExtraSystemRole(carolID)
			createProject(carolID, "carol", "payments")

			rootID := seedSuperuser("root", "Root")
			rec := doRequest("GET", "/api/projects", nil, rootID, "root")
			names := projectNamesInList(rec)
			Expect(names).To(HaveKey("billing"))
			Expect(names).To(HaveKey("payments"))
		})
	})
})
