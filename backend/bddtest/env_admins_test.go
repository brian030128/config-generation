package bddtest

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func addEnvAdmin(actorID int64, actorName, projectName, envName string, targetID int64) *httptest.ResponseRecorder {
	return doRequest("POST",
		fmt.Sprintf("/api/projects/%s/environments/%s/admins", projectName, envName),
		map[string]any{"user_id": targetID}, actorID, actorName)
}

func listEnvAdmins(actorID int64, actorName, projectName, envName string) []any {
	GinkgoHelper()
	rec := doRequest("GET",
		fmt.Sprintf("/api/projects/%s/environments/%s/admins", projectName, envName),
		nil, actorID, actorName)
	Expect(rec.Code).To(Equal(http.StatusOK))
	return decode[map[string]any](rec)["items"].([]any)
}

// envAdminUserIDs returns the set of user_ids that are env-admins of the env.
func envAdminUserIDs(actorID int64, actorName, projectName, envName string) map[int64]bool {
	ids := map[int64]bool{}
	for _, item := range listEnvAdmins(actorID, actorName, projectName, envName) {
		ids[int64(item.(map[string]any)["user_id"].(float64))] = true
	}
	return ids
}

var _ = Describe("Environment Admins", func() {
	var aliceID, bobID, carolID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith") // project admin / creator
		bobID = seedUser("bob", "Bob Jones")
		carolID = seedUser("carol", "Carol Davis")
		seedSystemRole(aliceID)
		createProject(aliceID, "alice", "billing")
	})

	Context("env creation through the workspace", func() {
		It("makes the creator the environment's first env-admin", func() {
			rec := doRequest("POST", "/api/workspace/billing/environments", map[string]any{
				"name": "staging",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			submitApproveMerge(aliceID, "alice", "billing")

			Expect(envAdminUserIDs(aliceID, "alice", "billing", "staging")).To(HaveKey(aliceID))
		})
	})

	Context("with staging and prod environments", func() {
		BeforeEach(func() {
			createEnvironment(aliceID, "alice", "billing", "staging")
			createEnvironment(aliceID, "alice", "billing", "prod")
		})

		It("lets a project admin grant env-admin", func() {
			rec := addEnvAdmin(aliceID, "alice", "billing", "staging", bobID)
			Expect(rec.Code).To(Equal(http.StatusCreated))
			Expect(envAdminUserIDs(aliceID, "alice", "billing", "staging")).To(HaveKey(bobID))
		})

		It("lets an env-admin manage that environment's values", func() {
			Expect(addEnvAdmin(aliceID, "alice", "billing", "staging", bobID).Code).To(Equal(http.StatusCreated))

			rec := doRequest("PUT", "/api/workspace/billing/envs/staging/values", map[string]any{
				"payload": map[string]any{"service_name": "billing"},
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		It("does not let an env-admin touch another environment's values", func() {
			Expect(addEnvAdmin(aliceID, "alice", "billing", "staging", bobID).Code).To(Equal(http.StatusCreated))

			rec := doRequest("PUT", "/api/workspace/billing/envs/prod/values", map[string]any{
				"payload": map[string]any{"service_name": "billing"},
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("does not let an env-admin create a brand-new environment", func() {
			Expect(addEnvAdmin(aliceID, "alice", "billing", "staging", bobID).Code).To(Equal(http.StatusCreated))

			rec := doRequest("POST", "/api/workspace/billing/environments", map[string]any{
				"name": "qa",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("lets an env-admin stage deletion of their environment but not others", func() {
			Expect(addEnvAdmin(aliceID, "alice", "billing", "staging", bobID).Code).To(Equal(http.StatusCreated))

			rec := doRequest("DELETE", "/api/workspace/billing/environments/staging", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))

			rec = doRequest("DELETE", "/api/workspace/billing/environments/prod", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("lets an env-admin grant env-admin to others (self-propagating)", func() {
			Expect(addEnvAdmin(aliceID, "alice", "billing", "staging", bobID).Code).To(Equal(http.StatusCreated))

			// Bob (a staging env-admin, not a project admin) grants Carol.
			Expect(addEnvAdmin(bobID, "bob", "billing", "staging", carolID).Code).To(Equal(http.StatusCreated))

			rec := doRequest("PUT", "/api/workspace/billing/envs/staging/values", map[string]any{
				"payload": map[string]any{"service_name": "billing"},
			}, carolID, "carol")
			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		It("forbids a non-admin from granting env-admin", func() {
			// Bob is neither a project admin nor an env-admin here.
			rec := addEnvAdmin(bobID, "bob", "billing", "staging", carolID)
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("refuses to remove the last env-admin", func() {
			Expect(addEnvAdmin(aliceID, "alice", "billing", "staging", bobID).Code).To(Equal(http.StatusCreated))

			rec := doRequest("DELETE",
				fmt.Sprintf("/api/projects/billing/environments/staging/admins/%d", bobID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("allows removing an env-admin when others remain", func() {
			Expect(addEnvAdmin(aliceID, "alice", "billing", "staging", bobID).Code).To(Equal(http.StatusCreated))
			Expect(addEnvAdmin(aliceID, "alice", "billing", "staging", carolID).Code).To(Equal(http.StatusCreated))

			rec := doRequest("DELETE",
				fmt.Sprintf("/api/projects/billing/environments/staging/admins/%d", bobID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNoContent))
			Expect(envAdminUserIDs(aliceID, "alice", "billing", "staging")).NotTo(HaveKey(bobID))
		})
	})
})
