package bddtest

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// setMemberPermissions PUTs a member's permissions, acting as the given actor.
func setMemberPermissions(actorID int64, actorName, projectName string, targetID int64, perms map[string]any) *httptest.ResponseRecorder {
	return doRequest("PUT",
		fmt.Sprintf("/api/projects/%s/members/%d/permissions", projectName, targetID),
		perms, actorID, actorName)
}

// envAccess is a convenience builder for one environments[] entry.
func envAccess(env string, read, write bool) map[string]any {
	return map[string]any{"env": env, "read": read, "write": write}
}

var _ = Describe("Member Permissions", func() {
	var aliceID, bobID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		bobID = seedUser("bob", "Bob Jones")
		seedSystemRole(aliceID)
		createProject(aliceID, "alice", "billing")
		addProjectMember(aliceID, "alice", "billing", bobID)
		createEnvironment(aliceID, "alice", "billing", "staging")
		createEnvironment(aliceID, "alice", "billing", "prod")
		createTemplate(aliceID, "alice", "billing", "app.yaml", "{{ .service_name }}")
		seedValues(aliceID, "billing", "staging", map[string]any{"service_name": "billing"})
		seedValues(aliceID, "billing", "prod", map[string]any{"service_name": "billing"})
	})

	It("a bare member cannot read any environment's values", func() {
		rec := doRequest("GET", "/api/projects/billing/envs/staging/values", nil, bobID, "bob")
		Expect(rec.Code).To(Equal(http.StatusForbidden))
		rec = doRequest("GET", "/api/projects/billing/envs/prod/values", nil, bobID, "bob")
		Expect(rec.Code).To(Equal(http.StatusForbidden))
	})

	It("read on staging lets the member see staging but NOT production", func() {
		rec := setMemberPermissions(aliceID, "alice", "billing", bobID, map[string]any{
			"environments": []map[string]any{envAccess("staging", true, false)},
		})
		Expect(rec.Code).To(Equal(http.StatusOK))

		rec = doRequest("GET", "/api/projects/billing/envs/staging/values", nil, bobID, "bob")
		Expect(rec.Code).To(Equal(http.StatusOK))

		rec = doRequest("GET", "/api/projects/billing/envs/prod/values", nil, bobID, "bob")
		Expect(rec.Code).To(Equal(http.StatusForbidden))
	})

	It("read-only on staging does not allow writing staging values", func() {
		_ = setMemberPermissions(aliceID, "alice", "billing", bobID, map[string]any{
			"environments": []map[string]any{envAccess("staging", true, false)},
		})

		rec := doRequest("PUT", "/api/workspace/billing/envs/staging/values", map[string]any{
			"payload": map[string]any{"service_name": "v2"},
		}, bobID, "bob")
		Expect(rec.Code).To(Equal(http.StatusForbidden))
	})

	It("write on staging lets the member edit staging values but not prod", func() {
		_ = setMemberPermissions(aliceID, "alice", "billing", bobID, map[string]any{
			"environments": []map[string]any{envAccess("staging", true, true)},
		})

		rec := doRequest("PUT", "/api/workspace/billing/envs/staging/values", map[string]any{
			"payload": map[string]any{"service_name": "v2"},
		}, bobID, "bob")
		Expect(rec.Code).To(Equal(http.StatusOK))

		rec = doRequest("PUT", "/api/workspace/billing/envs/prod/values", map[string]any{
			"payload": map[string]any{"service_name": "v2"},
		}, bobID, "bob")
		Expect(rec.Code).To(Equal(http.StatusForbidden))
	})

	It("write on an env lets the member bootstrap its first value set", func() {
		createEnvironment(aliceID, "alice", "billing", "qa") // no values yet
		_ = setMemberPermissions(aliceID, "alice", "billing", bobID, map[string]any{
			"environments": []map[string]any{envAccess("qa", true, true)},
		})

		rec := doRequest("PUT", "/api/workspace/billing/envs/qa/values", map[string]any{
			"payload": map[string]any{"service_name": "qa"},
		}, bobID, "bob")
		Expect(rec.Code).To(Equal(http.StatusOK))
	})

	It("write on an env does not let the member create a new environment", func() {
		_ = setMemberPermissions(aliceID, "alice", "billing", bobID, map[string]any{
			"environments": []map[string]any{envAccess("staging", true, true)},
		})

		rec := doRequest("POST", "/api/workspace/billing/environments", map[string]any{
			"name": "newenv",
		}, bobID, "bob")
		Expect(rec.Code).To(Equal(http.StatusForbidden))
	})

	It("write_templates is project-wide and can be revoked", func() {
		rec := setMemberPermissions(aliceID, "alice", "billing", bobID, map[string]any{
			"write_templates": true,
		})
		Expect(rec.Code).To(Equal(http.StatusOK))

		rec = doRequest("POST", "/api/workspace/billing/templates", map[string]any{
			"template_name": "new.yaml", "body": "new",
		}, bobID, "bob")
		Expect(rec.Code).To(Equal(http.StatusOK))

		// Revoke everything.
		_ = setMemberPermissions(aliceID, "alice", "billing", bobID, map[string]any{})
		rec = doRequest("POST", "/api/workspace/billing/templates", map[string]any{
			"template_name": "new2.yaml", "body": "new",
		}, bobID, "bob")
		Expect(rec.Code).To(Equal(http.StatusForbidden))
	})

	It("GET reflects the granted permissions", func() {
		_ = setMemberPermissions(aliceID, "alice", "billing", bobID, map[string]any{
			"read_templates": true,
			"environments": []map[string]any{
				envAccess("staging", true, true),
				envAccess("prod", true, false),
			},
		})

		rec := doRequest("GET",
			fmt.Sprintf("/api/projects/billing/members/%d/permissions", bobID), nil, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))
		body := decode[map[string]any](rec)
		Expect(body["read_templates"]).To(BeTrue())
		Expect(body["write_templates"]).To(BeFalse())

		levels := map[string][2]bool{} // env → {read, write}
		for _, item := range body["environments"].([]any) {
			e := item.(map[string]any)
			levels[e["env"].(string)] = [2]bool{e["read"].(bool), e["write"].(bool)}
		}
		Expect(levels["staging"]).To(Equal([2]bool{true, true}))
		Expect(levels["prod"]).To(Equal([2]bool{true, false}))
	})

	It("rejects an unknown environment in the body", func() {
		rec := setMemberPermissions(aliceID, "alice", "billing", bobID, map[string]any{
			"environments": []map[string]any{envAccess("ghost", true, false)},
		})
		Expect(rec.Code).To(Equal(http.StatusBadRequest))
	})

	It("a non-admin cannot set member permissions", func() {
		rec := setMemberPermissions(bobID, "bob", "billing", bobID, map[string]any{
			"environments": []map[string]any{envAccess("staging", true, false)},
		})
		Expect(rec.Code).To(Equal(http.StatusForbidden))
	})

	It("setting permissions for a non-member is rejected", func() {
		carolID := seedUser("carol", "Carol Davis")
		rec := setMemberPermissions(aliceID, "alice", "billing", carolID, map[string]any{
			"write_templates": true,
		})
		Expect(rec.Code).To(Equal(http.StatusNotFound))
	})
})
