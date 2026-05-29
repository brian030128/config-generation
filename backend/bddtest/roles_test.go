package bddtest

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Roles (global)", func() {
	var aliceID, bobID, rootID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		bobID = seedUser("bob", "Bob Jones")
		rootID = seedSuperuser("root", "Root")
		seedSystemRole(aliceID) // alice can create projects
	})

	Context("listing", func() {
		It("enriches member names and reports viewer_can_manage for a superuser", func() {
			createProject(aliceID, "alice", "billing")

			role := findRoleByName(rootID, "root", "billing_project_admin")
			Expect(role["is_auto_created"]).To(BeTrue())
			members := role["members"].([]any)
			Expect(members).To(HaveLen(1))
			member := members[0].(map[string]any)
			Expect(member["username"]).To(Equal("alice"))
			Expect(member["display_name"]).To(Equal("Alice Smith"))

			rec := doRequest("GET", "/api/roles", nil, rootID, "root")
			Expect(decode[map[string]any](rec)["viewer_can_manage"]).To(BeTrue())
		})

		It("reports viewer_can_manage=false for a non-superuser", func() {
			rec := doRequest("GET", "/api/roles", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["viewer_can_manage"]).To(BeFalse())
		})

		It("hides per-member managed roles", func() {
			createProject(aliceID, "alice", "billing")
			addProjectMember(aliceID, "alice", "billing", bobID)
			rec := doRequest("PUT", fmt.Sprintf("/api/projects/billing/members/%d/permissions", bobID), map[string]any{
				"read_templates":   true,
				"write_templates":  false,
				"delete_templates": false,
				"environments":     []any{},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			rec = doRequest("GET", "/api/roles", nil, rootID, "root")
			for _, item := range decode[map[string]any](rec)["items"].([]any) {
				Expect(item.(map[string]any)["name"].(string)).NotTo(HavePrefix("__member__:"))
			}
		})
	})

	Context("management is superuser-only", func() {
		It("lets a superuser create, assign, and delete a role", func() {
			createProject(aliceID, "alice", "billing")

			rec := doRequest("POST", "/api/roles", map[string]any{
				"name": "release_manager",
				"permissions": []map[string]any{
					{"action": "read", "resource": "project_templates", "key_project": "billing"},
				},
			}, rootID, "root")
			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["name"]).To(Equal("release_manager"))
			roleID := int64(body["id"].(float64))

			rec = doRequest("POST", fmt.Sprintf("/api/roles/%d/members", roleID), map[string]any{"user_id": bobID}, rootID, "root")
			Expect(rec.Code).To(Equal(http.StatusCreated))

			rec = doRequest("DELETE", fmt.Sprintf("/api/roles/%d", roleID), nil, rootID, "root")
			Expect(rec.Code).To(Equal(http.StatusNoContent))
		})

		It("forbids a non-superuser from creating a role", func() {
			rec := doRequest("POST", "/api/roles", map[string]any{"name": "x"}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("a role granting read:project lets a non-member see the project", func() {
			createProject(aliceID, "alice", "billing")

			rec := doRequest("POST", "/api/roles", map[string]any{
				"name": "billing_viewer",
				"permissions": []map[string]any{
					{"action": "read", "resource": "project", "key_project": "billing"},
					{"action": "read", "resource": "project_templates", "key_project": "billing"},
				},
			}, rootID, "root")
			Expect(rec.Code).To(Equal(http.StatusCreated))
			roleID := int64(decode[map[string]any](rec)["id"].(float64))

			By("bob (not a member) cannot read the project yet")
			rec = doRequest("GET", "/api/projects/billing", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))

			By("after assigning the read:project role, bob can read it")
			rec = doRequest("POST", fmt.Sprintf("/api/roles/%d/members", roleID), map[string]any{"user_id": bobID}, rootID, "root")
			Expect(rec.Code).To(Equal(http.StatusCreated))

			rec = doRequest("GET", "/api/projects/billing", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		It("rejects a duplicate role name", func() {
			rec := doRequest("POST", "/api/roles", map[string]any{"name": "dup"}, rootID, "root")
			Expect(rec.Code).To(Equal(http.StatusCreated))
			rec = doRequest("POST", "/api/roles", map[string]any{"name": "dup"}, rootID, "root")
			Expect(rec.Code).To(Equal(http.StatusConflict))
		})

		It("forbids a non-superuser from editing, assigning, or deleting roles", func() {
			roleID := createGlobalRoleAs(rootID, "root", "some-role", nil)

			rec := doRequest("PUT", fmt.Sprintf("/api/roles/%d/permissions", roleID),
				map[string]any{"permissions": []map[string]any{}}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusForbidden))

			rec = doRequest("POST", fmt.Sprintf("/api/roles/%d/members", roleID),
				map[string]any{"user_id": bobID}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusForbidden))

			rec = doRequest("DELETE", fmt.Sprintf("/api/roles/%d", roleID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("returns 404 for a superuser operating on a non-existent role", func() {
			rec := doRequest("DELETE", "/api/roles/999999", nil, rootID, "root")
			Expect(rec.Code).To(Equal(http.StatusNotFound))

			rec = doRequest("PUT", "/api/roles/999999/permissions",
				map[string]any{"permissions": []map[string]any{}}, rootID, "root")
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("replaces a custom role's permissions", func() {
			createProject(aliceID, "alice", "billing")
			roleID := createGlobalRoleAs(rootID, "root", "billing-role", []map[string]any{
				{"action": "read", "resource": "project_templates", "key_project": "billing"},
			})

			rec := doRequest("PUT", fmt.Sprintf("/api/roles/%d/permissions", roleID), map[string]any{
				"permissions": []map[string]any{
					{"action": "read", "resource": "project", "key_project": "billing"},
					{"action": "write", "resource": "project_templates", "key_project": "billing"},
				},
			}, rootID, "root")
			Expect(rec.Code).To(Equal(http.StatusNoContent))

			role := findRoleByName(rootID, "root", "billing-role")
			Expect(role["permissions"].([]any)).To(HaveLen(2))
		})
	})

	Context("project approval condition", func() {
		It("accepts a condition once the referenced role exists", func() {
			createProject(aliceID, "alice", "billing")
			createCustomRole(0, "", "", "release_manager", []map[string]any{
				{"action": "read", "resource": "project_templates", "key_project": "billing"},
			})

			rec := doRequest("PUT", "/api/projects/billing/approval-condition", map[string]any{
				"approval_condition": "1 x billing_project_admin AND 1 x release_manager",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["approval_condition"]).To(Equal("1 x billing_project_admin AND 1 x release_manager"))
		})

		It("rejects a condition referencing a non-existent role", func() {
			createProject(aliceID, "alice", "billing")
			rec := doRequest("PUT", "/api/projects/billing/approval-condition", map[string]any{
				"approval_condition": "1 x ghost_role",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("rejects a malformed condition with a dangling requirement", func() {
			createProject(aliceID, "alice", "billing")
			rec := doRequest("PUT", "/api/projects/billing/approval-condition", map[string]any{
				"approval_condition": "1 x billing_project_admin AND 1 x ",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("forbids a non-admin from updating the approval condition", func() {
			createProject(aliceID, "alice", "billing")
			addProjectMember(aliceID, "alice", "billing", bobID)
			rec := doRequest("PUT", "/api/projects/billing/approval-condition", map[string]any{
				"approval_condition": "1 x billing_project_admin",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})
	})

	Context("global values approval condition", func() {
		It("accepts a condition once the referenced role exists", func() {
			createGlobalValues(aliceID, "alice", "shared", map[string]any{"k": "v"})
			createCustomRole(0, "", "", "gv_reviewer", []map[string]any{
				{"action": "read", "resource": "global_values", "key_name": "shared"},
			})

			rec := doRequest("PUT", "/api/global-values/shared/approval-condition", map[string]any{
				"approval_condition": "1 x shared_gv_group_admin AND 1 x gv_reviewer",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["approval_condition"]).To(Equal("1 x shared_gv_group_admin AND 1 x gv_reviewer"))
		})

		It("rejects a GV condition referencing a non-existent role", func() {
			createGlobalValues(aliceID, "alice", "shared", map[string]any{"k": "v"})
			rec := doRequest("PUT", "/api/global-values/shared/approval-condition", map[string]any{
				"approval_condition": "1 x nobody",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})
})
