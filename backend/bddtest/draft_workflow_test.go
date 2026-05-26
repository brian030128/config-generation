package bddtest

import (
	"encoding/json"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Workspace Workflow (all changes through PR)", func() {
	var aliceID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		seedSystemRole(aliceID)
		createProject(aliceID, "alice", "my-service")
	})

	Context("staging environment creation", func() {
		It("stages an environment in the draft", func() {
			rec := doRequest("POST", "/api/workspace/my-service/environments", map[string]any{
				"name": "staging",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			body := decode[map[string]any](rec)
			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(1))
			change := changes[0].(map[string]any)
			Expect(change["object_type"]).To(Equal("environment"))
			Expect(change["operation"]).To(Equal("create"))
			Expect(change["environment_name"]).To(Equal("staging"))
		})

		It("can stage multiple environments in the same draft", func() {
			doRequest("POST", "/api/workspace/my-service/environments", map[string]any{"name": "staging"}, aliceID, "alice")

			rec := doRequest("POST", "/api/workspace/my-service/environments", map[string]any{"name": "production"}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			changes := decode[map[string]any](rec)["changes"].([]any)
			Expect(changes).To(HaveLen(2))
		})

		It("upserts environment changes by name", func() {
			doRequest("POST", "/api/workspace/my-service/environments", map[string]any{
				"name": "staging", "description": "old",
			}, aliceID, "alice")

			rec := doRequest("POST", "/api/workspace/my-service/environments", map[string]any{
				"name": "staging", "description": "updated",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			changes := decode[map[string]any](rec)["changes"].([]any)
			Expect(changes).To(HaveLen(1))
			change := changes[0].(map[string]any)
			var payload map[string]any
			Expect(json.Unmarshal([]byte(change["proposed_payload"].(string)), &payload)).To(Succeed())
			Expect(payload["description"]).To(Equal("updated"))
		})
	})

	Context("staging template creation", func() {
		It("stages a new template in the draft", func() {
			rec := doRequest("POST", "/api/workspace/my-service/templates", map[string]any{
				"template_name": "app.yaml",
				"body":          "service: {{ .service_name }}",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			change := decode[map[string]any](rec)["changes"].([]any)[0].(map[string]any)
			Expect(change["object_type"]).To(Equal("template"))
			Expect(change["operation"]).To(Equal("create"))
			Expect(change["template_name"]).To(Equal("app.yaml"))
		})

		It("rejects an empty body", func() {
			rec := doRequest("POST", "/api/workspace/my-service/templates", map[string]any{
				"template_name": "app.yaml", "body": "",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("can stage multiple templates", func() {
			doRequest("POST", "/api/workspace/my-service/templates", map[string]any{
				"template_name": "app.yaml", "body": "service: {{ .name }}",
			}, aliceID, "alice")

			rec := doRequest("POST", "/api/workspace/my-service/templates", map[string]any{
				"template_name": "worker.yaml", "body": "worker: {{ .worker }}",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["changes"].([]any)).To(HaveLen(2))
		})
	})

	Context("staging values for a draft environment", func() {
		It("allows staging values even when the environment only exists in the draft", func() {
			doRequest("POST", "/api/workspace/my-service/environments", map[string]any{"name": "staging"}, aliceID, "alice")

			rec := doRequest("PUT", "/api/workspace/my-service/envs/staging/values", map[string]any{
				"payload": map[string]any{"service_name": "my-service", "port": "8080"},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["changes"].([]any)).To(HaveLen(2))
		})

		It("sets base_version_id to 0 for values of a draft environment", func() {
			doRequest("POST", "/api/workspace/my-service/environments", map[string]any{"name": "staging"}, aliceID, "alice")

			rec := doRequest("PUT", "/api/workspace/my-service/envs/staging/values", map[string]any{
				"payload": map[string]any{"key": "val"},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			for _, c := range decode[map[string]any](rec)["changes"].([]any) {
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

		It("stages a new value set (base version 0)", func() {
			rec := doRequest("PUT", "/api/workspace/my-service/envs/staging/values", map[string]any{
				"payload": map[string]any{"key": "val"},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			change := decode[map[string]any](rec)["changes"].([]any)[0].(map[string]any)
			Expect(change["environment_name"]).To(Equal("staging"))
			Expect(change["operation"]).To(Equal("create"))
			Expect(change["base_version_id"]).To(BeEquivalentTo(0))
		})

		It("stages an edit to an existing value set with the correct base version", func() {
			seedValues(aliceID, "my-service", "staging", map[string]any{"key": "v1"})

			rec := doRequest("PUT", "/api/workspace/my-service/envs/staging/values", map[string]any{
				"payload": map[string]any{"key": "v2"},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			change := decode[map[string]any](rec)["changes"].([]any)[0].(map[string]any)
			Expect(change["operation"]).To(Equal("update"))
			Expect(change["base_version_id"]).To(BeEquivalentTo(1))
		})
	})

	Context("full workflow: stage everything, submit, approve, merge", func() {
		It("creates environments, templates, and values on merge", func() {
			By("staging an environment, template, and values")
			doRequest("POST", "/api/workspace/my-service/environments", map[string]any{"name": "staging"}, aliceID, "alice")
			doRequest("POST", "/api/workspace/my-service/templates", map[string]any{
				"template_name": "app.yaml", "body": "service: {{ .service_name }}\nport: {{ .port }}",
			}, aliceID, "alice")
			rec := doRequest("PUT", "/api/workspace/my-service/envs/staging/values", map[string]any{
				"payload": map[string]any{"service_name": "my-service", "port": "8080"},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(decode[map[string]any](rec)["changes"].([]any)).To(HaveLen(3))

			submitApproveMerge(aliceID, "alice", "my-service")

			By("verifying environment, template, and values are live")
			rec = doRequest("GET", "/api/projects/my-service/environments", nil, aliceID, "alice")
			envs := decode[map[string]any](rec)["items"].([]any)
			Expect(envs).To(HaveLen(1))
			Expect(envs[0].(map[string]any)["name"]).To(Equal("staging"))

			rec = doRequest("GET", "/api/projects/my-service/templates", nil, aliceID, "alice")
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(1))

			rec = doRequest("GET", "/api/projects/my-service/envs/staging/values", nil, aliceID, "alice")
			valsBody := decode[map[string]any](rec)
			Expect(valsBody["version_id"]).To(BeEquivalentTo(1))
			Expect(valsBody["payload"].(map[string]any)["service_name"]).To(Equal("my-service"))
		})
	})

	Context("deleting through the workspace", func() {
		BeforeEach(func() {
			createTemplate(aliceID, "alice", "my-service", "old.yaml", "legacy")
			createEnvironment(aliceID, "alice", "my-service", "staging")
			seedValues(aliceID, "my-service", "staging", map[string]any{"key": "v1"})
		})

		It("deletes a template on merge", func() {
			rec := doRequest("DELETE", "/api/workspace/my-service/templates/old.yaml", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			change := decode[map[string]any](rec)["changes"].([]any)[0].(map[string]any)
			Expect(change["operation"]).To(Equal("delete"))

			submitApproveMerge(aliceID, "alice", "my-service")

			rec = doRequest("GET", "/api/projects/my-service/templates", nil, aliceID, "alice")
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(0))
		})

		It("deletes a value set on merge", func() {
			rec := doRequest("DELETE", "/api/workspace/my-service/envs/staging/values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			submitApproveMerge(aliceID, "alice", "my-service")

			rec = doRequest("GET", "/api/projects/my-service/envs/staging/values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("deletes an environment and its values on merge", func() {
			rec := doRequest("DELETE", "/api/workspace/my-service/environments/staging", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			submitApproveMerge(aliceID, "alice", "my-service")

			rec = doRequest("GET", "/api/projects/my-service/environments", nil, aliceID, "alice")
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(0))
		})
	})

	Context("overlay reads (base + the caller's staged changes)", func() {
		BeforeEach(func() {
			createTemplate(aliceID, "alice", "my-service", "app.yaml", "v1 body")
		})

		It("shows the staged edit over the published base", func() {
			doRequest("PUT", "/api/workspace/my-service/templates/app.yaml", map[string]any{
				"body": "v2 staged body",
			}, aliceID, "alice")

			By("published base is unchanged")
			rec := doRequest("GET", "/api/projects/my-service/templates/app.yaml", nil, aliceID, "alice")
			Expect(decode[map[string]any](rec)["body"]).To(Equal("v1 body"))

			By("workspace overlay shows the proposed body")
			rec = doRequest("GET", "/api/workspace/my-service/templates/app.yaml", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["body"]).To(Equal("v2 staged body"))
			Expect(body["staged"]).To(BeTrue())
			Expect(body["operation"]).To(Equal("update"))
		})

		It("hides a staged-deleted template in the overlay list", func() {
			doRequest("DELETE", "/api/workspace/my-service/templates/app.yaml", nil, aliceID, "alice")

			rec := doRequest("GET", "/api/workspace/my-service/templates", nil, aliceID, "alice")
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(0))

			By("but the published list still has it")
			rec = doRequest("GET", "/api/projects/my-service/templates", nil, aliceID, "alice")
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(1))
		})
	})

	Context("change set / unstage", func() {
		It("removes a single staged change", func() {
			doRequest("POST", "/api/workspace/my-service/templates", map[string]any{
				"template_name": "app.yaml", "body": "body",
			}, aliceID, "alice")
			rec := doRequest("GET", "/api/workspace/my-service/changes", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			changes := decode[map[string]any](rec)["items"].([]any)
			Expect(changes).To(HaveLen(1))
			changeID := changes[0].(map[string]any)["id"].(float64)

			rec = doRequest("DELETE", fmt.Sprintf("/api/workspace/my-service/changes/%.0f", changeID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			rec = doRequest("GET", "/api/workspace/my-service/changes", nil, aliceID, "alice")
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(0))
		})
	})

	Context("discarding a workspace", func() {
		It("deletes the draft and its staged changes", func() {
			doRequest("POST", "/api/workspace/my-service/templates", map[string]any{
				"template_name": "app.yaml", "body": "body",
			}, aliceID, "alice")

			rec := doRequest("DELETE", "/api/workspace/my-service", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNoContent))

			By("a fresh workspace has no changes")
			rec = doRequest("GET", "/api/workspace/my-service/changes", nil, aliceID, "alice")
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(0))
		})
	})

	Context("draft visibility", func() {
		It("draft changes do not appear in the project API before merge", func() {
			doRequest("POST", "/api/workspace/my-service/environments", map[string]any{"name": "staging"}, aliceID, "alice")
			doRequest("POST", "/api/workspace/my-service/templates", map[string]any{
				"template_name": "app.yaml", "body": "body",
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/projects/my-service/environments", nil, aliceID, "alice")
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(0))

			rec = doRequest("GET", "/api/projects/my-service/templates", nil, aliceID, "alice")
			Expect(decode[map[string]any](rec)["count"]).To(BeEquivalentTo(0))
		})
	})
})
