package bddtest

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Multi-Project Workflow", func() {
	var (
		aliceID int64
		bobID   int64
		carolID int64
	)

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		bobID = seedUser("bob", "Bob Jones")
		carolID = seedUser("carol", "Carol Davis")
		seedSystemRole(aliceID)
		seedExtraSystemRole(carolID)
	})

	Context("a user authoring changes across multiple projects", func() {
		BeforeEach(func() {
			createProject(aliceID, "alice", "billing-service")
			createProject(carolID, "carol", "payments-service")

			createEnvironment(aliceID, "alice", "billing-service", "staging")
			createEnvironment(aliceID, "alice", "billing-service", "prod")
			createEnvironment(carolID, "carol", "payments-service", "staging")
			createEnvironment(carolID, "carol", "payments-service", "prod")

			// Bob: write on billing templates + billing staging values.
			billingDevRole := createCustomRole(aliceID, "alice", "billing-service", "billing-dev", []map[string]any{
				{"action": "write", "resource": "project_templates", "key_project": "billing-service"},
				{"action": "write", "resource": "project_values", "key_project": "billing-service", "key_env": "staging"},
			})
			assignUserToRole(aliceID, "alice", billingDevRole, bobID)

			// Bob: write on payments templates + all payments env values.
			paymentsDevRole := createCustomRole(carolID, "carol", "payments-service", "payments-dev", []map[string]any{
				{"action": "write", "resource": "project_templates", "key_project": "payments-service"},
				{"action": "write", "resource": "project_values", "key_project": "payments-service", "key_env": "*"},
			})
			assignUserToRole(carolID, "carol", paymentsDevRole, bobID)
		})

		It("bob authors an edit in billing that merges via alice's approval", func() {
			// Live baseline (created by the project admin). Both environments and
			// the referenced global values must stay renderable so bob's edit can
			// be submitted (the whole project is validated at submit time).
			createTemplate(aliceID, "alice", "billing-service", "app.yaml", "service: {{ .service_name }}")
			seedGlobalValues(aliceID, "shared_db", map[string]any{"host": "db.internal"})
			seedValues(aliceID, "billing-service", "staging", map[string]any{"service_name": "billing"})
			seedValues(aliceID, "billing-service", "prod", map[string]any{"service_name": "billing-prod", "db_host": "db.internal"})

			By("bob stages a template edit and a values edit")
			rec := doRequest("PUT", "/api/workspace/billing-service/templates/app.yaml", map[string]any{
				"body": "service: {{ .service_name }}\ndb_host: {{ .db_host }}",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			rec = doRequest("PUT", "/api/workspace/billing-service/envs/staging/values", map[string]any{
				"payload": map[string]any{"service_name": "billing-v2", "db_host": "${shared_db.host}"},
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))

			By("alice approves and bob merges")
			submitApproveMergeBy(bobID, "bob", aliceID, "alice", "billing-service")

			By("the edits are now live as v2")
			rec = doRequest("GET", "/api/projects/billing-service/templates/app.yaml", nil, bobID, "bob")
			Expect(decode[map[string]any](rec)["version_id"]).To(BeEquivalentTo(2))
			rec = doRequest("GET", "/api/projects/billing-service/envs/staging/values", nil, bobID, "bob")
			body := decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(2))
			Expect(body["payload"].(map[string]any)["service_name"]).To(Equal("billing-v2"))
		})

		It("bob's write permission is correctly scoped per project and env", func() {
			createTemplate(aliceID, "alice", "billing-service", "app.yaml", "v1")
			createTemplate(carolID, "carol", "payments-service", "app.yaml", "v1")

			By("bob can stage templates in both projects (write in both)")
			rec := doRequest("POST", "/api/workspace/billing-service/templates", map[string]any{
				"template_name": "worker.yaml", "body": "w",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			rec = doRequest("POST", "/api/workspace/payments-service/templates", map[string]any{
				"template_name": "cron.yaml", "body": "c",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))

			By("bob CANNOT stage billing prod values (only has staging)")
			rec = doRequest("PUT", "/api/workspace/billing-service/envs/prod/values", map[string]any{
				"payload": map[string]any{"env": "prod"},
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))

			By("bob CAN stage payments prod values (wildcard env)")
			seedValues(carolID, "payments-service", "prod", map[string]any{"env": "prod"})
			rec = doRequest("PUT", "/api/workspace/payments-service/envs/prod/values", map[string]any{
				"payload": map[string]any{"env": "prod-v2"},
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))

			By("bob CANNOT create roles (superuser-only) or delete either project")
			rec = doRequest("POST", "/api/roles", map[string]any{"name": "rogue"}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
			rec = doRequest("DELETE", "/api/projects/payments-service", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("concurrent version histories remain independent across projects", func() {
			createTemplate(aliceID, "alice", "billing-service", "app.yaml", "billing v1")
			createTemplate(carolID, "carol", "payments-service", "app.yaml", "payments v1")

			for i := 2; i <= 4; i++ {
				appendTemplateVersion(aliceID, "alice", "billing-service", "app.yaml", fmt.Sprintf("billing v%d", i))
			}
			for i := 2; i <= 3; i++ {
				appendTemplateVersion(carolID, "carol", "payments-service", "app.yaml", fmt.Sprintf("payments v%d", i))
			}

			rec := doRequest("GET", "/api/projects/billing-service/templates/app.yaml", nil, aliceID, "alice")
			Expect(decode[map[string]any](rec)["version_id"]).To(BeEquivalentTo(4))
			rec = doRequest("GET", "/api/projects/payments-service/templates/app.yaml", nil, carolID, "carol")
			Expect(decode[map[string]any](rec)["version_id"]).To(BeEquivalentTo(3))

			rec = doRequest("GET", "/api/projects/billing-service/templates/app.yaml/versions", nil, aliceID, "alice")
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(4))
			rec = doRequest("GET", "/api/projects/payments-service/templates/app.yaml/versions", nil, carolID, "carol")
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(3))
		})

		It("shared global value references are stored independently per project", func() {
			createGlobalValues(aliceID, "alice", "shared_db", map[string]any{"host": "db.internal"})
			createGlobalValues(aliceID, "alice", "shared_redis", map[string]any{"host": "redis.internal"})

			seedValues(aliceID, "billing-service", "staging", map[string]any{
				"db_host": "${shared_db.host}", "db_password": "${shared_db.password}",
			})
			seedValues(carolID, "payments-service", "staging", map[string]any{
				"db_host": "${shared_db.host}", "cache_host": "${shared_redis.host}",
			})

			rec := doRequest("GET", "/api/projects/billing-service/envs/staging/values", nil, aliceID, "alice")
			billingPayload := decode[map[string]any](rec)["payload"].(map[string]any)
			Expect(billingPayload["db_host"]).To(Equal("${shared_db.host}"))
			Expect(billingPayload).NotTo(HaveKey("cache_host"))

			rec = doRequest("GET", "/api/projects/payments-service/envs/staging/values", nil, carolID, "carol")
			paymentsPayload := decode[map[string]any](rec)["payload"].(map[string]any)
			Expect(paymentsPayload["cache_host"]).To(Equal("${shared_redis.host}"))
			Expect(paymentsPayload).NotTo(HaveKey("db_password"))
		})

		It("deleting one project does not affect the other", func() {
			createTemplate(aliceID, "alice", "billing-service", "app.yaml", "billing")
			createTemplate(carolID, "carol", "payments-service", "app.yaml", "payments")

			rec := doRequest("DELETE", "/api/projects/billing-service", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNoContent))

			// alice lost membership on delete; confirm the project is gone via a
			// superuser (who bypasses read:project).
			rootID := seedSuperuser("root", "Root")
			rec = doRequest("GET", "/api/projects/billing-service", nil, rootID, "root")
			Expect(rec.Code).To(Equal(http.StatusNotFound))

			rec = doRequest("GET", "/api/projects/payments-service/templates/app.yaml", nil, carolID, "carol")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["body"]).To(Equal("payments"))
		})
	})

	Context("multiple admins collaborating with role delegation", func() {
		BeforeEach(func() {
			createProject(aliceID, "alice", "billing-service")
			createProject(carolID, "carol", "payments-service")
		})

		It("alice and carol each delegate to bob with different scopes", func() {
			r1 := createCustomRole(aliceID, "alice", "billing-service", "billing-dev", []map[string]any{
				{"action": "write", "resource": "project_templates", "key_project": "billing-service"},
			})
			assignUserToRole(aliceID, "alice", r1, bobID)

			r2 := createCustomRole(carolID, "carol", "payments-service", "payments-reader", []map[string]any{
				{"action": "read", "resource": "project_templates", "key_project": "payments-service"},
			})
			assignUserToRole(carolID, "carol", r2, bobID)

			By("bob can stage templates in billing (write)")
			rec := doRequest("POST", "/api/workspace/billing-service/templates", map[string]any{
				"template_name": "app.yaml", "body": "billing template",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))

			By("bob CANNOT stage templates in payments (read-only)")
			rec = doRequest("POST", "/api/workspace/payments-service/templates", map[string]any{
				"template_name": "app.yaml", "body": "payments template",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))

			By("carol publishes a template; bob can read it")
			createTemplate(carolID, "carol", "payments-service", "app.yaml", "payments template")
			rec = doRequest("GET", "/api/projects/payments-service/templates/app.yaml", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["body"]).To(Equal("payments template"))
		})
	})
})
