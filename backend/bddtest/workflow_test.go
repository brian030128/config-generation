package bddtest

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Multi-Project Workflow", func() {
	var (
		aliceID   int64
		bobID     int64
		carolID   int64
		stagingID float64
		prodID    float64
	)

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		bobID = seedUser("bob", "Bob Jones")
		carolID = seedUser("carol", "Carol Davis")
		seedSystemRole(aliceID)
		seedExtraSystemRole(carolID)
	})

	Context("a user authoring changes across multiple projects in a single session", func() {
		BeforeEach(func() {
			// Alice owns billing-service
			createProject(aliceID, "alice", "billing-service")

			// Carol owns payments-service
			createProject(carolID, "carol", "payments-service")

			// Create environments per project
			env := createEnvironment(aliceID, "alice", "billing-service", "staging")
			stagingID = env["id"].(float64)
			env = createEnvironment(aliceID, "alice", "billing-service", "prod")
			prodID = env["id"].(float64)
			createEnvironment(carolID, "carol", "payments-service", "staging")
			createEnvironment(carolID, "carol", "payments-service", "prod")

			// Alice grants bob write access to billing templates and staging values
			billingDevRole := createCustomRole(aliceID, "alice", "billing-service", "billing-dev", []map[string]any{
				{"action": "write", "resource": "project_templates", "key_project": "billing-service"},
				{"action": "write", "resource": "project_values", "key_project": "billing-service", "key_env": "staging"},
			})
			assignUserToRole(aliceID, "alice", billingDevRole, bobID)

			// Carol grants bob write access to payments templates and all env values
			paymentsDevRole := createCustomRole(carolID, "carol", "payments-service", "payments-dev", []map[string]any{
				{"action": "write", "resource": "project_templates", "key_project": "payments-service"},
				{"action": "write", "resource": "project_values", "key_project": "payments-service", "key_env": "*"},
			})
			assignUserToRole(carolID, "carol", paymentsDevRole, bobID)

			// Set up shared global values
			createGlobalValues(aliceID, "alice", "shared_db", map[string]any{
				"host":     "db.internal",
				"port":     5432,
				"username": "app",
				"password": "s3cret",
			})
			createGlobalValues(aliceID, "alice", "shared_redis", map[string]any{
				"host": "redis.internal",
				"port": 6379,
			})
		})

		It("bob creates templates and values across both projects", func() {
			By("creating billing-service templates")
			createTemplate(bobID, "bob", "billing-service", "app.yaml", `service: {{ .service_name }}
db_host: {{ .db_host }}`)
			createTemplate(bobID, "bob", "billing-service", "worker.yaml", `worker: {{ .worker_name }}
concurrency: {{ .concurrency }}`)

			By("creating payments-service templates")
			createTemplate(bobID, "bob", "payments-service", "app.yaml", `service: {{ .service_name }}
stripe_key: {{ .stripe_key }}`)
			createTemplate(bobID, "bob", "payments-service", "cron.yaml", `schedule: {{ .schedule }}
task: {{ .task }}`)

			By("alice bootstraps billing staging values (bob can't create, only write)")
			doRequest("POST", "/api/projects/billing-service/values", map[string]any{
				"environment_id": stagingID,
				"payload": map[string]any{
					"service_name": "billing",
					"db_host":      "${shared_db.host}",
					"worker_name":  "invoice-worker",
					"concurrency":  4,
				},
			}, aliceID, "alice")

			By("carol bootstraps payments staging and prod values")
			doRequest("POST", "/api/projects/payments-service/values", map[string]any{
				"environment_id": stagingID,
				"payload": map[string]any{
					"service_name": "payments",
					"stripe_key":   "sk_test_xxx",
				},
			}, carolID, "carol")
			doRequest("POST", "/api/projects/payments-service/values", map[string]any{
				"environment_id": prodID,
				"payload": map[string]any{
					"service_name": "payments",
					"stripe_key":   "sk_live_xxx",
				},
			}, carolID, "carol")

			By("bob updates billing staging values (append v2)")
			rec := doRequest("POST", "/api/projects/billing-service/envs/staging/values/versions", map[string]any{
				"payload": map[string]any{
					"service_name": "billing-v2",
					"db_host":      "${shared_db.host}",
					"db_password":  "${shared_db.password}",
					"worker_name":  "invoice-worker",
					"concurrency":  4,
				},
				"commit_message": "Add db_password reference",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusCreated))
			Expect(decode[map[string]any](rec)["version_id"]).To(BeEquivalentTo(2))

			By("bob updates payments staging values (append v2)")
			rec = doRequest("POST", "/api/projects/payments-service/envs/staging/values/versions", map[string]any{
				"payload": map[string]any{
					"service_name": "payments-v2",
					"stripe_key":   "sk_test_updated",
					"redis_host":   "${shared_redis.host}",
				},
				"commit_message": "Add redis cache config",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusCreated))
			Expect(decode[map[string]any](rec)["version_id"]).To(BeEquivalentTo(2))

			By("bob also updates payments prod values")
			rec = doRequest("POST", "/api/projects/payments-service/envs/prod/values/versions", map[string]any{
				"payload": map[string]any{
					"service_name": "payments-v2",
					"stripe_key":   "sk_live_updated",
					"redis_host":   "${shared_redis.host}",
				},
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusCreated))

			By("bob updates a template in billing-service (append v2)")
			rec = doRequest("POST", "/api/projects/billing-service/templates/app.yaml/versions", map[string]any{
				"body": `service: {{ .service_name }}
db_host: {{ .db_host }}
db_password: {{ .db_password }}`,
				"commit_message": "Add db_password to template",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusCreated))
			Expect(decode[map[string]any](rec)["version_id"]).To(BeEquivalentTo(2))

			By("verifying billing-service state")
			rec = doRequest("GET", "/api/projects/billing-service/templates/app.yaml", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["version_id"]).To(BeEquivalentTo(2))

			rec = doRequest("GET", "/api/projects/billing-service/envs/staging/values", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(2))
			payload := body["payload"].(map[string]any)
			Expect(payload["db_password"]).To(Equal("${shared_db.password}"))

			By("verifying payments-service state")
			rec = doRequest("GET", "/api/projects/payments-service/envs/staging/values", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body = decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(2))
			payload = body["payload"].(map[string]any)
			Expect(payload["redis_host"]).To(Equal("${shared_redis.host}"))

			rec = doRequest("GET", "/api/projects/payments-service/envs/prod/values", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["version_id"]).To(BeEquivalentTo(2))
		})

		It("bob's permissions are correctly scoped per project", func() {
			createTemplate(aliceID, "alice", "billing-service", "app.yaml", "v1")
			createTemplate(carolID, "carol", "payments-service", "app.yaml", "v1")

			// Alice bootstraps billing prod values
			doRequest("POST", "/api/projects/billing-service/values", map[string]any{
				"environment_id": prodID,
				"payload": map[string]any{"env": "prod"},
			}, aliceID, "alice")

			By("bob CANNOT write billing prod values (only has staging)")
			rec := doRequest("POST", "/api/projects/billing-service/envs/prod/values/versions", map[string]any{
				"payload": map[string]any{"env": "prod-v2"},
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))

			By("bob CAN write payments prod values (has wildcard env)")
			carolBootstraps := doRequest("POST", "/api/projects/payments-service/values", map[string]any{
				"environment_id": prodID,
				"payload": map[string]any{"env": "prod"},
			}, carolID, "carol")
			Expect(carolBootstraps.Code).To(Equal(http.StatusCreated))

			rec = doRequest("POST", "/api/projects/payments-service/envs/prod/values/versions", map[string]any{
				"payload": map[string]any{"env": "prod-v2"},
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusCreated))

			By("bob CANNOT create roles in either project")
			rec = doRequest("POST", "/api/projects/billing-service/roles", map[string]any{
				"name": "rogue-role",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))

			rec = doRequest("POST", "/api/projects/payments-service/roles", map[string]any{
				"name": "rogue-role",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))

			By("bob CANNOT delete either project")
			rec = doRequest("DELETE", "/api/projects/billing-service", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
			rec = doRequest("DELETE", "/api/projects/payments-service", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("concurrent version histories remain independent across projects", func() {
			createTemplate(aliceID, "alice", "billing-service", "app.yaml", "billing v1")
			createTemplate(carolID, "carol", "payments-service", "app.yaml", "payments v1")

			By("bob appends 3 versions to billing app.yaml")
			for i := 2; i <= 4; i++ {
				rec := doRequest("POST", "/api/projects/billing-service/templates/app.yaml/versions", map[string]any{
					"body":           fmt.Sprintf("billing v%d", i),
					"commit_message": fmt.Sprintf("billing update %d", i),
				}, bobID, "bob")
				Expect(rec.Code).To(Equal(http.StatusCreated))
			}

			By("bob appends 2 versions to payments app.yaml")
			for i := 2; i <= 3; i++ {
				rec := doRequest("POST", "/api/projects/payments-service/templates/app.yaml/versions", map[string]any{
					"body":           fmt.Sprintf("payments v%d", i),
					"commit_message": fmt.Sprintf("payments update %d", i),
				}, bobID, "bob")
				Expect(rec.Code).To(Equal(http.StatusCreated))
			}

			By("billing app.yaml is at v4")
			rec := doRequest("GET", "/api/projects/billing-service/templates/app.yaml", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["version_id"]).To(BeEquivalentTo(4))

			By("payments app.yaml is at v3")
			rec = doRequest("GET", "/api/projects/payments-service/templates/app.yaml", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["version_id"]).To(BeEquivalentTo(3))

			By("billing version history has 4 entries")
			rec = doRequest("GET", "/api/projects/billing-service/templates/app.yaml/versions", nil, bobID, "bob")
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(4))

			By("payments version history has 3 entries")
			rec = doRequest("GET", "/api/projects/payments-service/templates/app.yaml/versions", nil, bobID, "bob")
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(3))
		})

		It("shared global values are referenced by multiple projects independently", func() {
			createTemplate(aliceID, "alice", "billing-service", "app.yaml", "{{ .db_host }}")
			createTemplate(carolID, "carol", "payments-service", "app.yaml", "{{ .db_host }}")

			By("alice creates billing staging values referencing shared_db")
			doRequest("POST", "/api/projects/billing-service/values", map[string]any{
				"environment_id": stagingID,
				"payload": map[string]any{
					"db_host":     "${shared_db.host}",
					"db_password": "${shared_db.password}",
				},
			}, aliceID, "alice")

			By("carol creates payments staging values referencing same shared_db")
			doRequest("POST", "/api/projects/payments-service/values", map[string]any{
				"environment_id": stagingID,
				"payload": map[string]any{
					"db_host":    "${shared_db.host}",
					"cache_host": "${shared_redis.host}",
				},
			}, carolID, "carol")

			By("both projects store their references independently")
			rec := doRequest("GET", "/api/projects/billing-service/envs/staging/values", nil, aliceID, "alice")
			billingPayload := decode[map[string]any](rec)["payload"].(map[string]any)
			Expect(billingPayload["db_host"]).To(Equal("${shared_db.host}"))
			Expect(billingPayload["db_password"]).To(Equal("${shared_db.password}"))
			Expect(billingPayload).NotTo(HaveKey("cache_host"))

			rec = doRequest("GET", "/api/projects/payments-service/envs/staging/values", nil, carolID, "carol")
			paymentsPayload := decode[map[string]any](rec)["payload"].(map[string]any)
			Expect(paymentsPayload["db_host"]).To(Equal("${shared_db.host}"))
			Expect(paymentsPayload["cache_host"]).To(Equal("${shared_redis.host}"))
			Expect(paymentsPayload).NotTo(HaveKey("db_password"))
		})

		It("deleting one project does not affect the other", func() {
			createTemplate(aliceID, "alice", "billing-service", "app.yaml", "billing")
			createTemplate(carolID, "carol", "payments-service", "app.yaml", "payments")

			By("alice deletes billing-service")
			rec := doRequest("DELETE", "/api/projects/billing-service", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNoContent))

			By("billing-service is gone")
			rec = doRequest("GET", "/api/projects/billing-service", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNotFound))

			By("payments-service is unaffected")
			rec = doRequest("GET", "/api/projects/payments-service", nil, carolID, "carol")
			Expect(rec.Code).To(Equal(http.StatusOK))

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
			By("alice gives bob full template write on billing")
			r1 := createCustomRole(aliceID, "alice", "billing-service", "billing-dev", []map[string]any{
				{"action": "write", "resource": "project_templates", "key_project": "billing-service"},
				{"action": "write", "resource": "project_values", "key_project": "billing-service", "key_env": "*"},
			})
			assignUserToRole(aliceID, "alice", r1, bobID)

			By("carol gives bob read-only on payments")
			r2 := createCustomRole(carolID, "carol", "payments-service", "payments-reader", []map[string]any{
				{"action": "read", "resource": "project_templates", "key_project": "payments-service"},
				{"action": "read", "resource": "project_values", "key_project": "payments-service", "key_env": "*"},
			})
			assignUserToRole(carolID, "carol", r2, bobID)

			By("bob creates templates in billing (write)")
			rec := doRequest("POST", "/api/projects/billing-service/templates", map[string]any{
				"template_name": "app.yaml", "body": "billing template",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusCreated))

			By("bob CANNOT create templates in payments (read-only)")
			rec = doRequest("POST", "/api/projects/payments-service/templates", map[string]any{
				"template_name": "app.yaml", "body": "payments template",
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusForbidden))

			By("carol creates a template, bob can read it")
			createTemplate(carolID, "carol", "payments-service", "app.yaml", "payments template")
			rec = doRequest("GET", "/api/projects/payments-service/templates/app.yaml", nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["body"]).To(Equal("payments template"))
		})
	})
})
