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

	Context("creating a template", func() {
		It("returns 201 with version_id=1 using the doc example app.yaml template", func() {
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

			rec := doRequest("POST", "/api/projects/billing-service/templates", map[string]any{
				"template_name":  "app.yaml",
				"body":           templateBody,
				"commit_message": "Initial app.yaml template",
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["template_name"]).To(Equal("app.yaml"))
			Expect(body["version_id"]).To(BeEquivalentTo(1))
			Expect(body["body"]).To(Equal(templateBody))
		})

		It("rejects duplicate template names with 409", func() {
			createTemplate(aliceID, "alice", "billing-service", "app.yaml", "body1")

			rec := doRequest("POST", "/api/projects/billing-service/templates", map[string]any{
				"template_name": "app.yaml",
				"body":          "body2",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusConflict))
		})

		It("rejects empty body with 400", func() {
			rec := doRequest("POST", "/api/projects/billing-service/templates", map[string]any{
				"template_name": "app.yaml",
				"body":          "",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("rejects empty template_name with 400", func() {
			rec := doRequest("POST", "/api/projects/billing-service/templates", map[string]any{
				"template_name": "",
				"body":          "some body",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("full-copy versioning", func() {
		BeforeEach(func() {
			createTemplate(aliceID, "alice", "billing-service", "app.yaml", "v1 body")
		})

		It("appends version 2 with new body", func() {
			rec := doRequest("POST", "/api/projects/billing-service/templates/app.yaml/versions", map[string]any{
				"body":           "v2 body",
				"commit_message": "Update template",
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(2))
			Expect(body["body"]).To(Equal("v2 body"))
		})

		It("preserves version 1 immutably after appending version 2", func() {
			doRequest("POST", "/api/projects/billing-service/templates/app.yaml/versions", map[string]any{
				"body": "v2 body",
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/projects/billing-service/templates/app.yaml/versions/1", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["body"]).To(Equal("v1 body"))
			Expect(body["version_id"]).To(BeEquivalentTo(1))
		})

		It("returns the latest version by default", func() {
			doRequest("POST", "/api/projects/billing-service/templates/app.yaml/versions", map[string]any{
				"body": "v2 body",
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/projects/billing-service/templates/app.yaml", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(2))
		})

		It("lists all versions in descending order", func() {
			doRequest("POST", "/api/projects/billing-service/templates/app.yaml/versions", map[string]any{
				"body": "v2 body",
			}, aliceID, "alice")
			doRequest("POST", "/api/projects/billing-service/templates/app.yaml/versions", map[string]any{
				"body": "v3 body",
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/projects/billing-service/templates/app.yaml/versions", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(3))
			items := body["items"].([]any)
			first := items[0].(map[string]any)
			Expect(first["version_id"]).To(BeEquivalentTo(3))
			last := items[2].(map[string]any)
			Expect(last["version_id"]).To(BeEquivalentTo(1))
		})
	})

	Context("listing templates for a project", func() {
		It("returns the latest version of each distinct template", func() {
			createTemplate(aliceID, "alice", "billing-service", "app.yaml", "v1 body")
			createTemplate(aliceID, "alice", "billing-service", "database.conf", "db config")

			// Append v2 of app.yaml
			doRequest("POST", "/api/projects/billing-service/templates/app.yaml/versions", map[string]any{
				"body": "v2 body",
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/projects/billing-service/templates", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(2))

			items := body["items"].([]any)
			templateMap := make(map[string]float64)
			for _, item := range items {
				t := item.(map[string]any)
				templateMap[t["template_name"].(string)] = t["version_id"].(float64)
			}
			Expect(templateMap["app.yaml"]).To(BeEquivalentTo(2))
			Expect(templateMap["database.conf"]).To(BeEquivalentTo(1))
		})
	})
})
