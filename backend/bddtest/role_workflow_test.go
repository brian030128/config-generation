package bddtest

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// createGlobalRoleAs creates a global role as the given actor (must be superuser)
// and returns its id.
func createGlobalRoleAs(actorID int64, actorName, name string, perms []map[string]any) int64 {
	GinkgoHelper()
	rec := doRequest("POST", "/api/roles", map[string]any{
		"name":        name,
		"permissions": perms,
	}, actorID, actorName)
	Expect(rec.Code).To(Equal(http.StatusCreated))
	return int64(decode[map[string]any](rec)["id"].(float64))
}

func assignRoleAs(actorID int64, actorName string, roleID, targetUserID int64) {
	GinkgoHelper()
	rec := doRequest("POST", fmt.Sprintf("/api/roles/%d/members", roleID),
		map[string]any{"user_id": targetUserID}, actorID, actorName)
	Expect(rec.Code).To(Equal(http.StatusCreated))
}

var _ = Describe("Role-gated PR approval", func() {
	var aliceID, bobID, rootID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		bobID = seedUser("bob", "Bob Jones")
		rootID = seedSuperuser("root", "Root")
		seedSystemRole(aliceID) // alice can create projects
	})

	// Stages a template change in alice's workspace and submits it, returning the PR id.
	stageAndSubmit := func() float64 {
		GinkgoHelper()
		rec := doRequest("POST", "/api/workspace/billing/templates", map[string]any{
			"template_name": "app.yaml",
			"body":          "hello: world",
		}, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))

		rec = doRequest("GET", "/api/workspace/billing", nil, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))
		prID := decode[map[string]any](rec)["id"].(float64)

		rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/submit", prID),
			map[string]any{"title": "add app.yaml"}, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))
		return prID
	}

	It("merges only once both a project_admin and a release_manager approve", func() {
		createProject(aliceID, "alice", "billing")

		// A superuser creates release_manager (with project read so its holder can
		// see the project) and assigns bob.
		rmID := createGlobalRoleAs(rootID, "root", "release_manager", []map[string]any{
			{"action": "read", "resource": "project", "key_project": "billing"},
		})
		assignRoleAs(rootID, "root", rmID, bobID)

		// Require both roles to approve.
		rec := doRequest("PUT", "/api/projects/billing/approval-condition", map[string]any{
			"approval_condition": "1 x billing_project_admin AND 1 x release_manager",
		}, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))

		prID := stageAndSubmit()

		By("only the project admin has approved -> condition unmet, merge blocked")
		rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/approve", prID), nil, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(decode[map[string]any](rec)["status"]).To(Equal("open"))

		rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/merge", prID), nil, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusConflict))

		By("once the release_manager also approves -> condition met, merge succeeds")
		rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/approve", prID), nil, bobID, "bob")
		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(decode[map[string]any](rec)["status"]).To(Equal("approved"))

		rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/merge", prID), nil, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))
	})

	It("reverts to open when a required approver withdraws", func() {
		createProject(aliceID, "alice", "billing")
		rmID := createGlobalRoleAs(rootID, "root", "release_manager", []map[string]any{
			{"action": "read", "resource": "project", "key_project": "billing"},
		})
		assignRoleAs(rootID, "root", rmID, bobID)
		rec := doRequest("PUT", "/api/projects/billing/approval-condition", map[string]any{
			"approval_condition": "1 x billing_project_admin AND 1 x release_manager",
		}, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))

		prID := stageAndSubmit()
		doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/approve", prID), nil, aliceID, "alice")
		rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/approve", prID), nil, bobID, "bob")
		Expect(decode[map[string]any](rec)["status"]).To(Equal("approved"))

		By("bob withdraws -> the release_manager requirement is no longer met")
		rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/withdraw-approval", prID), nil, bobID, "bob")
		Expect(rec.Code).To(Equal(http.StatusOK))

		rec = doRequest("GET", fmt.Sprintf("/api/pull-requests/%.0f", prID), nil, aliceID, "alice")
		Expect(decode[map[string]any](rec)["status"]).To(Equal("open"))

		rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/merge", prID), nil, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusConflict))
	})
})
