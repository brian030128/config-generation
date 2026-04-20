package bddtest

import (
	"encoding/json"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Draft PR Workflow (all changes through PR)", func() {
	var aliceID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		seedSystemRole(aliceID)
		createProject(aliceID, "alice", "my-service")
	})

	Context("staging environment creation", func() {
		It("stages an environment in the draft", func() {
			rec := doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "environment",
				"proposed_payload": `{"name":"staging"}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(1))
			change := changes[0].(map[string]any)
			Expect(change["object_type"]).To(Equal("environment"))
			Expect(change["environment_name"]).To(Equal("staging"))
		})

		It("can stage multiple environments in the same draft", func() {
			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "environment",
				"proposed_payload": `{"name":"staging"}`,
			}, aliceID, "alice")

			rec := doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "environment",
				"proposed_payload": `{"name":"production"}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(2))
		})

		It("upserts environment changes by name", func() {
			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "environment",
				"proposed_payload": `{"name":"staging","description":"old"}`,
			}, aliceID, "alice")

			rec := doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "environment",
				"proposed_payload": `{"name":"staging","description":"updated"}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(1))
			change := changes[0].(map[string]any)
			var payload map[string]any
			json.Unmarshal([]byte(change["proposed_payload"].(string)), &payload)
			Expect(payload["description"]).To(Equal("updated"))
		})
	})

	Context("staging template creation", func() {
		It("stages a new template in the draft", func() {
			rec := doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "template",
				"template_name":    "app.yaml",
				"proposed_payload": "service: {{ .service_name }}",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(1))
			change := changes[0].(map[string]any)
			Expect(change["object_type"]).To(Equal("template"))
			Expect(change["template_name"]).To(Equal("app.yaml"))
		})

		It("can stage multiple templates", func() {
			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "template",
				"template_name":    "app.yaml",
				"proposed_payload": "service: {{ .name }}",
			}, aliceID, "alice")

			rec := doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "template",
				"template_name":    "worker.yaml",
				"proposed_payload": "worker: {{ .worker }}",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(2))
		})
	})

	Context("staging values for a draft environment", func() {
		It("allows staging values even when the environment only exists in the draft", func() {
			// Stage environment creation
			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "environment",
				"proposed_payload": `{"name":"staging"}`,
			}, aliceID, "alice")

			// Stage values for the not-yet-created environment
			rec := doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "values",
				"environment_name": "staging",
				"proposed_payload": `{"service_name":"my-service","port":"8080"}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(2))
		})

		It("sets base_version_id to 0 for values of a draft environment", func() {
			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "environment",
				"proposed_payload": `{"name":"staging"}`,
			}, aliceID, "alice")

			rec := doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "values",
				"environment_name": "staging",
				"proposed_payload": `{"key":"val"}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			changes := body["changes"].([]any)
			for _, c := range changes {
				change := c.(map[string]any)
				if change["object_type"] == "values" {
					Expect(change["base_version_id"]).To(BeEquivalentTo(0))
				}
			}
		})
	})

	Context("staging values for an existing environment", func() {
		BeforeEach(func() {
			createEnvironment(aliceID, "alice", "my-service", "staging")
		})

		It("stages values with correct base version", func() {
			rec := doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "values",
				"environment_name": "staging",
				"proposed_payload": `{"key":"val"}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(1))
			change := changes[0].(map[string]any)
			Expect(change["environment_name"]).To(Equal("staging"))
			Expect(change["base_version_id"]).To(BeEquivalentTo(0))
		})
	})

	Context("full draft workflow: stage everything, submit, approve, merge", func() {
		It("creates environments, templates, and values on merge", func() {
			By("staging an environment")
			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "environment",
				"proposed_payload": `{"name":"staging"}`,
			}, aliceID, "alice")

			By("staging a template")
			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "template",
				"template_name":    "app.yaml",
				"proposed_payload": "service: {{ .service_name }}\nport: {{ .port }}",
			}, aliceID, "alice")

			By("staging values for the draft environment")
			rec := doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "values",
				"environment_name": "staging",
				"proposed_payload": `{"service_name":"my-service","port":"8080"}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(3))
			prID := body["id"].(float64)

			By("submitting the draft PR")
			rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/submit", prID), map[string]any{
				"title":       "Initial setup",
				"description": "Add staging env, app template, and values",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			By("approving the PR")
			rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/approve", prID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			By("merging the PR")
			rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/merge", prID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			mergedPR := decode[map[string]any](rec)
			Expect(mergedPR["status"]).To(Equal("merged"))

			By("verifying environment was created")
			rec = doRequest("GET", "/api/projects/my-service/environments", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			envBody := decode[map[string]any](rec)
			Expect(envBody["count"]).To(BeEquivalentTo(1))
			envs := envBody["items"].([]any)
			Expect(envs[0].(map[string]any)["name"]).To(Equal("staging"))

			By("verifying template was created")
			rec = doRequest("GET", "/api/projects/my-service/templates", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			tmplBody := decode[map[string]any](rec)
			Expect(tmplBody["count"]).To(BeEquivalentTo(1))

			By("verifying values were created")
			rec = doRequest("GET", "/api/projects/my-service/envs/staging/values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			valsBody := decode[map[string]any](rec)
			Expect(valsBody["version_id"]).To(BeEquivalentTo(1))
			payload := valsBody["payload"].(map[string]any)
			Expect(payload["service_name"]).To(Equal("my-service"))
			Expect(payload["port"]).To(Equal("8080"))
		})

		It("creates multiple environments and values in a single PR", func() {
			By("staging two environments")
			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "environment",
				"proposed_payload": `{"name":"staging"}`,
			}, aliceID, "alice")
			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "environment",
				"proposed_payload": `{"name":"production"}`,
			}, aliceID, "alice")

			By("staging a template")
			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "template",
				"template_name":    "config.yaml",
				"proposed_payload": "env: {{ .env }}",
			}, aliceID, "alice")

			By("staging values for both environments")
			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "values",
				"environment_name": "staging",
				"proposed_payload": `{"env":"staging"}`,
			}, aliceID, "alice")
			rec := doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "values",
				"environment_name": "production",
				"proposed_payload": `{"env":"production"}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			prID := body["id"].(float64)
			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(5))

			By("submitting, approving, and merging")
			doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/submit", prID), map[string]any{
				"title": "Multi-env setup",
			}, aliceID, "alice")
			doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/approve", prID), nil, aliceID, "alice")
			rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/merge", prID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			By("verifying both environments exist")
			rec = doRequest("GET", "/api/projects/my-service/environments", nil, aliceID, "alice")
			envBody := decode[map[string]any](rec)
			Expect(envBody["count"]).To(BeEquivalentTo(2))

			By("verifying values for staging")
			rec = doRequest("GET", "/api/projects/my-service/envs/staging/values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["payload"].(map[string]any)["env"]).To(Equal("staging"))

			By("verifying values for production")
			rec = doRequest("GET", "/api/projects/my-service/envs/production/values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["payload"].(map[string]any)["env"]).To(Equal("production"))
		})
	})

	Context("draft visibility", func() {
		It("draft shows in the workspace endpoint", func() {
			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "template",
				"template_name":    "app.yaml",
				"proposed_payload": "body",
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/workspace/my-service/draft", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["status"]).To(Equal("draft"))
			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(1))
		})

		It("draft changes do not appear in project API before merge", func() {
			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "environment",
				"proposed_payload": `{"name":"staging"}`,
			}, aliceID, "alice")

			doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "template",
				"template_name":    "app.yaml",
				"proposed_payload": "body",
			}, aliceID, "alice")

			By("environments should be empty")
			rec := doRequest("GET", "/api/projects/my-service/environments", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(0))

			By("templates should be empty (only direct-created show)")
			rec = doRequest("GET", "/api/projects/my-service/templates", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(0))
		})
	})

	Context("project-level variables include staged templates", func() {
		It("returns variables from merged templates", func() {
			createTemplate(aliceID, "alice", "my-service", "app.yaml", "{{ .host }}\n{{ .port | default \"8080\" }}")

			rec := doRequest("GET", "/api/projects/my-service/variables", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			vars := body["variables"].([]any)
			Expect(vars).To(HaveLen(2))
		})
	})

	Context("editing template after initial merge", func() {
		It("stages an edit to an existing template", func() {
			createTemplate(aliceID, "alice", "my-service", "app.yaml", "v1 body")

			rec := doRequest("POST", "/api/workspace/my-service/stage", map[string]any{
				"object_type":      "template",
				"template_name":    "app.yaml",
				"proposed_payload": "v2 body with {{ .new_var }}",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(1))
			change := changes[0].(map[string]any)
			Expect(change["base_version_id"]).To(BeEquivalentTo(1))
		})
	})
})
