package bddtest

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Template Variables", func() {
	var aliceID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		seedSystemRole(aliceID)
		createProject(aliceID, "alice", "my-service")
	})

	Context("extracting variables from a simple template", func() {
		BeforeEach(func() {
			createTemplate(aliceID, "alice", "my-service", "app.yaml",
				`service: {{ .service_name }}
db_host: {{ .db_host }}
port: {{ .port }}`)
		})

		It("returns all referenced variables in order", func() {
			rec := doRequest("GET", "/api/projects/my-service/templates/app.yaml/variables", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			vars := body["variables"].([]any)
			Expect(vars).To(HaveLen(3))
			Expect(vars[0].(map[string]any)["name"]).To(Equal("service_name"))
			Expect(vars[1].(map[string]any)["name"]).To(Equal("db_host"))
			Expect(vars[2].(map[string]any)["name"]).To(Equal("port"))
		})
	})

	Context("extracting variables with Sprig default values", func() {
		BeforeEach(func() {
			createTemplate(aliceID, "alice", "my-service", "app.yaml",
				`service: {{ .service_name }}
port: {{ .port | default 8080 }}
debug: {{ .debug | default "false" }}`)
		})

		It("returns variables with their default values", func() {
			rec := doRequest("GET", "/api/projects/my-service/templates/app.yaml/variables", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			vars := body["variables"].([]any)
			Expect(vars).To(HaveLen(3))

			svcName := vars[0].(map[string]any)
			Expect(svcName["name"]).To(Equal("service_name"))
			Expect(svcName).NotTo(HaveKey("default"))

			port := vars[1].(map[string]any)
			Expect(port["name"]).To(Equal("port"))
			Expect(port["default"]).To(Equal("8080"))

			debug := vars[2].(map[string]any)
			Expect(debug["name"]).To(Equal("debug"))
			Expect(debug["default"]).To(Equal("false"))
		})
	})

	Context("extracting variables with Sprig functions in pipeline", func() {
		BeforeEach(func() {
			createTemplate(aliceID, "alice", "my-service", "app.yaml",
				`name: {{ .service_name | upper }}
version: {{ .version | default "1.0" | quote }}`)
		})

		It("extracts the variable names regardless of pipeline functions", func() {
			rec := doRequest("GET", "/api/projects/my-service/templates/app.yaml/variables", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			vars := body["variables"].([]any)
			Expect(vars).To(HaveLen(2))
			Expect(vars[0].(map[string]any)["name"]).To(Equal("service_name"))
			Expect(vars[1].(map[string]any)["name"]).To(Equal("version"))
			Expect(vars[1].(map[string]any)["default"]).To(Equal("1.0"))
		})
	})

	Context("deduplication of repeated variables", func() {
		BeforeEach(func() {
			createTemplate(aliceID, "alice", "my-service", "app.yaml",
				`first: {{ .name }}
second: {{ .name }}
third: {{ .name | upper }}`)
		})

		It("returns each variable only once", func() {
			rec := doRequest("GET", "/api/projects/my-service/templates/app.yaml/variables", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			vars := body["variables"].([]any)
			Expect(vars).To(HaveLen(1))
			Expect(vars[0].(map[string]any)["name"]).To(Equal("name"))
		})
	})

	Context("template with conditionals", func() {
		BeforeEach(func() {
			createTemplate(aliceID, "alice", "my-service", "app.yaml",
				`{{ if .enable_debug }}debug: true{{ end }}
host: {{ .host }}`)
		})

		It("extracts variables from inside conditionals", func() {
			rec := doRequest("GET", "/api/projects/my-service/templates/app.yaml/variables", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			vars := body["variables"].([]any)
			Expect(vars).To(HaveLen(2))
			Expect(vars[0].(map[string]any)["name"]).To(Equal("enable_debug"))
			Expect(vars[1].(map[string]any)["name"]).To(Equal("host"))
		})
	})

	Context("template with no variables", func() {
		BeforeEach(func() {
			createTemplate(aliceID, "alice", "my-service", "static.yaml",
				`key: hardcoded_value
another: 42`)
		})

		It("returns empty variables list", func() {
			rec := doRequest("GET", "/api/projects/my-service/templates/static.yaml/variables", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			vars := body["variables"].([]any)
			Expect(vars).To(BeEmpty())
		})
	})

	Context("uses the latest template version", func() {
		BeforeEach(func() {
			createTemplate(aliceID, "alice", "my-service", "app.yaml", `old: {{ .old_var }}`)
			seedTemplateWritePermission(aliceID, "my-service")
			appendTemplateVersion(aliceID, "alice", "my-service", "app.yaml", `new: {{ .new_var }}`)
		})

		It("returns variables from the latest version only", func() {
			rec := doRequest("GET", "/api/projects/my-service/templates/app.yaml/variables", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			vars := body["variables"].([]any)
			Expect(vars).To(HaveLen(1))
			Expect(vars[0].(map[string]any)["name"]).To(Equal("new_var"))
		})
	})

	Context("error cases", func() {
		It("returns 403 for nonexistent project (permission check)", func() {
			rec := doRequest("GET", "/api/projects/nope/templates/app.yaml/variables", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("returns 404 for nonexistent template", func() {
			rec := doRequest("GET", "/api/projects/my-service/templates/nope.yaml/variables", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})
})
