package bddtest

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Templates", func() {
	var aliceID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		seedSystemRole(aliceID)
		createProject(aliceID, "alice", "billing-service")
	})

	Context("creating a template via the workspace", func() {
		It("stages then publishes the doc example app.yaml template on merge", func() {
			templateBody := `service: {{ .service_name }}
environment: {{ .env }}
database:
  host: {{ .db_host }}
  port: {{ .db_port }}
  user: {{ .db_user }}
  password: {{ .db_password }}
features:
{{- range $k, $v := .feature_flags }}
  {{ $k }}: {{ $v }}
{{- end }}`

			rec := doRequest("POST", "/api/workspace/billing-service/templates", map[string]any{
				"template_name":  "app.yaml",
				"body":           templateBody,
				"commit_message": "Initial app.yaml template",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			By("staging an environment and values so the template can render")
			doRequest("POST", "/api/workspace/billing-service/environments", map[string]any{"name": "staging"}, aliceID, "alice")
			rec = doRequest("PUT", "/api/workspace/billing-service/envs/staging/values", map[string]any{
				"payload": map[string]any{
					"service_name":  "billing",
					"env":           "staging",
					"db_host":       "db.internal",
					"db_port":       "5432",
					"db_user":       "app",
					"db_password":   "s3cret",
					"feature_flags": map[string]any{"new_checkout": true},
				},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			submitApproveMerge(aliceID, "alice", "billing-service")

			rec = doRequest("GET", "/api/projects/billing-service/templates/app.yaml", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["template_name"]).To(Equal("app.yaml"))
			Expect(body["version_id"]).To(BeEquivalentTo(1))
			Expect(body["body"]).To(Equal(templateBody))
		})

		It("rejects empty body with 400", func() {
			rec := doRequest("POST", "/api/workspace/billing-service/templates", map[string]any{
				"template_name": "app.yaml", "body": "",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("rejects empty template_name with 400", func() {
			rec := doRequest("POST", "/api/workspace/billing-service/templates", map[string]any{
				"template_name": "", "body": "some body",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("full-copy versioning (read model)", func() {
		BeforeEach(func() {
			createTemplate(aliceID, "alice", "billing-service", "app.yaml", "v1 body")
		})

		It("exposes the appended version as the latest", func() {
			appendTemplateVersion(aliceID, "alice", "billing-service", "app.yaml", "v2 body")

			rec := doRequest("GET", "/api/projects/billing-service/templates/app.yaml", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(2))
			Expect(body["body"]).To(Equal("v2 body"))
		})

		It("preserves version 1 immutably after appending version 2", func() {
			appendTemplateVersion(aliceID, "alice", "billing-service", "app.yaml", "v2 body")

			rec := doRequest("GET", "/api/projects/billing-service/templates/app.yaml/versions/1", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["body"]).To(Equal("v1 body"))
			Expect(body["version_id"]).To(BeEquivalentTo(1))
		})

		It("lists all versions in descending order", func() {
			appendTemplateVersion(aliceID, "alice", "billing-service", "app.yaml", "v2 body")
			appendTemplateVersion(aliceID, "alice", "billing-service", "app.yaml", "v3 body")

			rec := doRequest("GET", "/api/projects/billing-service/templates/app.yaml/versions", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(3))
			items := body["items"].([]any)
			Expect(items[0].(map[string]any)["version_id"]).To(BeEquivalentTo(3))
			Expect(items[2].(map[string]any)["version_id"]).To(BeEquivalentTo(1))
		})
	})

	Context("listing templates for a project", func() {
		It("returns the latest version of each distinct template", func() {
			createTemplate(aliceID, "alice", "billing-service", "app.yaml", "v1 body")
			createTemplate(aliceID, "alice", "billing-service", "database.conf", "db config")
			appendTemplateVersion(aliceID, "alice", "billing-service", "app.yaml", "v2 body")

			rec := doRequest("GET", "/api/projects/billing-service/templates", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(2))

			templateMap := make(map[string]float64)
			for _, item := range body["items"].([]any) {
				t := item.(map[string]any)
				templateMap[t["template_name"].(string)] = t["version_id"].(float64)
			}
			Expect(templateMap["app.yaml"]).To(BeEquivalentTo(2))
			Expect(templateMap["database.conf"]).To(BeEquivalentTo(1))
		})
	})
})
