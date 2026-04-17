package bddtest

import (
	"context"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func createCustomRole(userID int64, username, projectName, roleName string, permissions []map[string]any) float64 {
	rec := doRequest("POST", "/api/projects/"+projectName+"/roles", map[string]any{
		"name":        roleName,
		"permissions": permissions,
	}, userID, username)
	Expect(rec.Code).To(Equal(http.StatusCreated))
	body := decode[map[string]any](rec)
	return body["id"].(float64)
}

func assignUserToRole(adminID int64, adminUsername string, roleID float64, targetUserID int64) {
	rec := doRequest("POST", fmt.Sprintf("/api/roles/%v/members", int64(roleID)), map[string]any{
		"user_id": targetUserID,
	}, adminID, adminUsername)
	Expect(rec.Code).To(Equal(http.StatusCreated))
}

func seedExtraSystemRole(userID int64) {
	var roleID int64
	err := testDB.QueryRowContext(context.Background(),
		`INSERT INTO roles (name, is_auto_created) VALUES ($1, false) RETURNING id`,
		fmt.Sprintf("system_admin_%v", userID),
	).Scan(&roleID)
	Expect(err).NotTo(HaveOccurred())

	_, err = testDB.ExecContext(context.Background(),
		`INSERT INTO role_permissions (role_id, action, resource) VALUES ($1, 'create', 'project')`,
		roleID,
	)
	Expect(err).NotTo(HaveOccurred())

	_, err = testDB.ExecContext(context.Background(),
		`INSERT INTO user_roles (user_id, role_id, granted_by) VALUES ($1, $2, $1)`,
		userID, roleID,
	)
	Expect(err).NotTo(HaveOccurred())
}

func getAutoCreatedRoleID(userID int64, username, projectName string) float64 {
	rec := doRequest("GET", "/api/projects/"+projectName+"/roles", nil, userID, username)
	Expect(rec.Code).To(Equal(http.StatusOK))
	body := decode[map[string]any](rec)
	items := body["items"].([]any)
	for _, item := range items {
		role := item.(map[string]any)
		if role["is_auto_created"].(bool) {
			return role["id"].(float64)
		}
	}
	Fail("auto-created role not found")
	return 0
}

var _ = Describe("Permission Model", func() {
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

	Context("write implies read (spec section 2.2)", func() {
		BeforeEach(func() {
			createTemplate(aliceID, "alice", "billing", "app.yaml", "template body")

			roleID := createCustomRole(aliceID, "alice", "billing", "billing-writer", []map[string]any{
				{"action": "write", "resource": "project_templates", "key_project": "billing"},
			})
			assignUserToRole(aliceID, "alice", roleID, bobID)
		})

		It("allows bob to read templates (implied by write)", func() {
			rec := doRequest("GET", "/api/projects/billing/templates/app.yaml", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		It("allows bob to write templates (direct grant)", func() {
			rec := doRequest("POST", "/api/projects/billing/templates", map[string]any{
				"template_name": "new.yaml",
				"body":          "new template",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusCreated))
		})

		It("denies read on a different project without permission", func() {
			carolID := seedUser("carol", "Carol Davis")
			seedExtraSystemRole(carolID)
			createProject(carolID, "carol", "payments")
			createTemplate(carolID, "carol", "payments", "app.yaml", "body")

			rec := doRequest("GET", "/api/projects/payments/templates/app.yaml", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})
	})

	Context("create:env_values implies write:project_values (spec section 3.2)", func() {
		var stagingID float64

		BeforeEach(func() {
			env := createEnvironment(aliceID, "alice", "staging")
			stagingID = env["id"].(float64)
			createTemplate(aliceID, "alice", "billing", "app.yaml", "{{ .service_name }}")
		})

		It("allows alice to create new value sets (create:env_values from project_admin)", func() {
			rec := doRequest("POST", "/api/projects/billing/values", map[string]any{
				"template_name":  "app.yaml",
				"environment_id": stagingID,
				"payload":        map[string]any{"service_name": "billing"},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusCreated))
		})

		It("allows alice to append versions (write implied by create)", func() {
			doRequest("POST", "/api/projects/billing/values", map[string]any{
				"template_name":  "app.yaml",
				"environment_id": stagingID,
				"payload":        map[string]any{"service_name": "billing-v1"},
			}, aliceID, "alice")

			rec := doRequest("POST", "/api/projects/billing/templates/app.yaml/envs/staging/values/versions", map[string]any{
				"payload": map[string]any{"service_name": "billing-v2"},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusCreated))
		})

		It("allows alice to read values (read implied transitively)", func() {
			doRequest("POST", "/api/projects/billing/values", map[string]any{
				"template_name":  "app.yaml",
				"environment_id": stagingID,
				"payload":        map[string]any{"service_name": "billing"},
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/projects/billing/templates/app.yaml/envs/staging/values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
		})
	})

	Context("wildcard matching (spec section 2.2)", func() {
		var stagingID, prodID float64

		BeforeEach(func() {
			env := createEnvironment(aliceID, "alice", "staging")
			stagingID = env["id"].(float64)
			env = createEnvironment(aliceID, "alice", "prod")
			prodID = env["id"].(float64)
			createTemplate(aliceID, "alice", "billing", "app.yaml", "{{ .env }}")

			doRequest("POST", "/api/projects/billing/values", map[string]any{
				"template_name": "app.yaml", "environment_id": stagingID,
				"payload": map[string]any{"env": "staging"},
			}, aliceID, "alice")
			doRequest("POST", "/api/projects/billing/values", map[string]any{
				"template_name": "app.yaml", "environment_id": prodID,
				"payload": map[string]any{"env": "prod"},
			}, aliceID, "alice")

			roleID := createCustomRole(aliceID, "alice", "billing", "billing-all-envs", []map[string]any{
				{"action": "write", "resource": "project_values", "key_project": "billing", "key_env": "*"},
			})
			assignUserToRole(aliceID, "alice", roleID, bobID)
		})

		It("allows bob to write values for staging (covered by wildcard)", func() {
			rec := doRequest("POST", "/api/projects/billing/templates/app.yaml/envs/staging/values/versions", map[string]any{
				"payload": map[string]any{"env": "staging-v2"},
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusCreated))
		})

		It("allows bob to write values for prod (also covered by wildcard)", func() {
			rec := doRequest("POST", "/api/projects/billing/templates/app.yaml/envs/prod/values/versions", map[string]any{
				"payload": map[string]any{"env": "prod-v2"},
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusCreated))
		})

		It("allows bob to read values for any env (write implies read)", func() {
			rec := doRequest("GET", "/api/projects/billing/templates/app.yaml/envs/staging/values", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))

			rec = doRequest("GET", "/api/projects/billing/templates/app.yaml/envs/prod/values", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
		})
	})

	Context("Alice and Bob scenario (spec section 5.1 and 5.2)", func() {
		var stagingID, euProdID float64

		BeforeEach(func() {
			env := createEnvironment(aliceID, "alice", "staging")
			stagingID = env["id"].(float64)
			env = createEnvironment(aliceID, "alice", "eu-prod")
			euProdID = env["id"].(float64)
			createTemplate(aliceID, "alice", "billing", "app.yaml", "{{ .env }}")

			roleID := createCustomRole(aliceID, "alice", "billing", "billing-staging-writer", []map[string]any{
				{"action": "write", "resource": "project_values", "key_project": "billing", "key_env": "staging"},
			})
			assignUserToRole(aliceID, "alice", roleID, bobID)
		})

		It("alice can create eu-prod values (she has create:env_values)", func() {
			rec := doRequest("POST", "/api/projects/billing/values", map[string]any{
				"template_name":  "app.yaml",
				"environment_id": euProdID,
				"payload":        map[string]any{"env": "eu-prod"},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusCreated))
		})

		It("bob can edit existing staging values (he has write:project_values(billing,staging))", func() {
			doRequest("POST", "/api/projects/billing/values", map[string]any{
				"template_name":  "app.yaml",
				"environment_id": stagingID,
				"payload":        map[string]any{"env": "staging"},
			}, aliceID, "alice")

			rec := doRequest("POST", "/api/projects/billing/templates/app.yaml/envs/staging/values/versions", map[string]any{
				"payload": map[string]any{"env": "staging-v2"},
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusCreated))
		})

		It("bob cannot create new values for eu-prod (no create:env_values)", func() {
			rec := doRequest("POST", "/api/projects/billing/values", map[string]any{
				"template_name":  "app.yaml",
				"environment_id": euProdID,
				"payload":        map[string]any{"env": "eu-prod"},
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("bob cannot write values for eu-prod (no write:project_values(billing,eu-prod))", func() {
			doRequest("POST", "/api/projects/billing/values", map[string]any{
				"template_name":  "app.yaml",
				"environment_id": euProdID,
				"payload":        map[string]any{"env": "eu-prod"},
			}, aliceID, "alice")

			rec := doRequest("POST", "/api/projects/billing/templates/app.yaml/envs/eu-prod/values/versions", map[string]any{
				"payload": map[string]any{"env": "eu-prod-v2"},
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})
	})

	Context("project admin scope is limited (spec section 5.3)", func() {
		var carolID int64

		BeforeEach(func() {
			carolID = seedUser("carol", "Carol Davis")
			seedExtraSystemRole(carolID)
			createProject(carolID, "carol", "payments")
		})

		It("carol cannot manage roles in billing (alice's project)", func() {
			rec := doRequest("POST", "/api/projects/billing/roles", map[string]any{
				"name": "my-role",
			}, carolID, "carol")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("alice cannot manage roles in payments (carol's project)", func() {
			rec := doRequest("POST", "/api/projects/payments/roles", map[string]any{
				"name": "my-role",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})
	})

	Context("role management", func() {
		It("project admin can create custom roles", func() {
			rec := doRequest("POST", "/api/projects/billing/roles", map[string]any{
				"name": "billing-reader",
				"permissions": []map[string]any{
					{"action": "read", "resource": "project_templates", "key_project": "billing"},
				},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["name"]).To(Equal("billing-reader"))
			Expect(body["is_auto_created"]).To(BeFalse())
		})

		It("project admin can assign users to roles", func() {
			roleID := createCustomRole(aliceID, "alice", "billing", "billing-reader", []map[string]any{
				{"action": "read", "resource": "project_templates", "key_project": "billing"},
			})

			rec := doRequest("POST", fmt.Sprintf("/api/roles/%v/members", int64(roleID)), map[string]any{
				"user_id": bobID,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusCreated))
		})

		It("cannot edit auto-created role permissions", func() {
			adminRoleID := getAutoCreatedRoleID(aliceID, "alice", "billing")

			rec := doRequest("PUT", fmt.Sprintf("/api/roles/%v/permissions", int64(adminRoleID)), map[string]any{
				"permissions": []map[string]any{
					{"action": "read", "resource": "project_templates", "key_project": "billing"},
				},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("cannot delete auto-created roles", func() {
			adminRoleID := getAutoCreatedRoleID(aliceID, "alice", "billing")

			rec := doRequest("DELETE", fmt.Sprintf("/api/roles/%v", int64(adminRoleID)), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("cannot remove the last member of the project admin role", func() {
			adminRoleID := getAutoCreatedRoleID(aliceID, "alice", "billing")

			rec := doRequest("DELETE", fmt.Sprintf("/api/roles/%v/members/%v", int64(adminRoleID), aliceID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})
})
