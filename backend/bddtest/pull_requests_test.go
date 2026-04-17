package bddtest

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pull Requests", func() {
	var aliceID, bobID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		bobID = seedUser("bob", "Bob Jones")
	})

	Context("listing pull requests", func() {
		It("returns empty list when none exist", func() {
			rec := doRequest("GET", "/api/pull-requests", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(0))
			Expect(body["items"]).To(BeEmpty())
		})
	})

	Context("creating a pull request for global values", func() {
		BeforeEach(func() {
			createGlobalValues(aliceID, "alice", "test_db_values", map[string]any{
				"host": "db.internal",
				"port": 5432,
			})
		})

		It("returns 201 with PR and change details", func() {
			rec := doRequest("POST", "/api/pull-requests", map[string]any{
				"title":              "Update DB host",
				"description":        "Change to new cluster",
				"object_type":        "global_values",
				"global_values_name": "test_db_values",
				"proposed_payload":   `{"host":"new-db.internal","port":5433}`,
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["title"]).To(Equal("Update DB host"))
			Expect(body["description"]).To(Equal("Change to new cluster"))
			Expect(body["status"]).To(Equal("open"))
			Expect(body["project_id"]).To(BeNil())
			Expect(body["author_id"]).To(BeEquivalentTo(aliceID))

			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(1))
			change := changes[0].(map[string]any)
			Expect(change["object_type"]).To(Equal("global_values"))
			Expect(change["global_values_name"]).To(Equal("test_db_values"))
			Expect(change["base_version_id"]).To(BeEquivalentTo(1))
		})

		It("records the correct base_version_id after multiple versions", func() {
			seedGlobalValuesPermission(aliceID, "test_db_values")
			doRequest("POST", "/api/global-values/test_db_values/versions", map[string]any{
				"payload": map[string]any{"host": "v2-db", "port": 5432},
			}, aliceID, "alice")

			rec := doRequest("POST", "/api/pull-requests", map[string]any{
				"title":              "Update to v3",
				"object_type":        "global_values",
				"global_values_name": "test_db_values",
				"proposed_payload":   `{"host":"v3-db","port":5432}`,
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			changes := body["changes"].([]any)
			change := changes[0].(map[string]any)
			Expect(change["base_version_id"]).To(BeEquivalentTo(2))
		})

		It("rejects missing title with 400", func() {
			rec := doRequest("POST", "/api/pull-requests", map[string]any{
				"object_type":        "global_values",
				"global_values_name": "test_db_values",
				"proposed_payload":   `{"host":"x"}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("rejects unsupported object_type with 400", func() {
			rec := doRequest("POST", "/api/pull-requests", map[string]any{
				"title":            "Bad type",
				"object_type":      "template",
				"proposed_payload": `{"body":"x"}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("rejects missing global_values_name with 400", func() {
			rec := doRequest("POST", "/api/pull-requests", map[string]any{
				"title":            "No name",
				"object_type":      "global_values",
				"proposed_payload": `{"host":"x"}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("rejects missing proposed_payload with 400", func() {
			rec := doRequest("POST", "/api/pull-requests", map[string]any{
				"title":              "No payload",
				"object_type":        "global_values",
				"global_values_name": "test_db_values",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 404 when global values entry does not exist", func() {
			rec := doRequest("POST", "/api/pull-requests", map[string]any{
				"title":              "Nonexistent",
				"object_type":        "global_values",
				"global_values_name": "does_not_exist",
				"proposed_payload":   `{"x":1}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	Context("getting a pull request", func() {
		var prID float64

		BeforeEach(func() {
			createGlobalValues(aliceID, "alice", "test_db_values", map[string]any{
				"host": "db.internal",
				"port": 5432,
			})

			rec := doRequest("POST", "/api/pull-requests", map[string]any{
				"title":              "Update DB",
				"object_type":        "global_values",
				"global_values_name": "test_db_values",
				"proposed_payload":   `{"host":"new-db","port":5432}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			prID = body["id"].(float64)
		})

		It("returns the PR with its changes", func() {
			rec := doRequest("GET", fmt.Sprintf("/api/pull-requests/%.0f", prID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["title"]).To(Equal("Update DB"))
			Expect(body["status"]).To(Equal("open"))

			changes := body["changes"].([]any)
			Expect(changes).To(HaveLen(1))
		})

		It("returns 404 for nonexistent PR", func() {
			rec := doRequest("GET", "/api/pull-requests/99999", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	Context("closing a pull request", func() {
		var prID float64

		BeforeEach(func() {
			createGlobalValues(aliceID, "alice", "test_db_values", map[string]any{
				"host": "db.internal",
				"port": 5432,
			})

			rec := doRequest("POST", "/api/pull-requests", map[string]any{
				"title":              "PR to close",
				"object_type":        "global_values",
				"global_values_name": "test_db_values",
				"proposed_payload":   `{"host":"new-db","port":5432}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			prID = body["id"].(float64)
		})

		It("closes an open PR and returns 200", func() {
			rec := doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/close", prID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["status"]).To(Equal("closed"))
			Expect(body["closed_at"]).NotTo(BeNil())
		})

		It("shows closed status when fetching after close", func() {
			rec := doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/close", prID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			rec = doRequest("GET", fmt.Sprintf("/api/pull-requests/%.0f", prID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["status"]).To(Equal("closed"))
		})

		It("returns 409 when closing an already closed PR", func() {
			rec := doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/close", prID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))

			rec = doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/close", prID), nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusConflict))
		})

		It("allows another user to close the PR", func() {
			rec := doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/close", prID), nil, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["status"]).To(Equal("closed"))
		})

		It("returns 404 for nonexistent PR", func() {
			rec := doRequest("POST", "/api/pull-requests/99999/close", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 400 for invalid PR ID", func() {
			rec := doRequest("POST", "/api/pull-requests/abc/close", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("closed PR appears in closed tab filter", func() {
			doRequest("POST", fmt.Sprintf("/api/pull-requests/%.0f/close", prID), nil, aliceID, "alice")

			rec := doRequest("GET", "/api/pull-requests", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			items := body["items"].([]any)
			Expect(items).To(HaveLen(1))
			pr := items[0].(map[string]any)
			Expect(pr["status"]).To(Equal("closed"))
		})
	})

	Context("listing pull requests with filter", func() {
		BeforeEach(func() {
			createGlobalValues(aliceID, "alice", "test_db_values", map[string]any{"host": "db"})
			createGlobalValues(aliceID, "alice", "other_values", map[string]any{"key": "val"})

			rec := doRequest("POST", "/api/pull-requests", map[string]any{
				"title":              "PR for test_db",
				"object_type":        "global_values",
				"global_values_name": "test_db_values",
				"proposed_payload":   `{"host":"new"}`,
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusCreated))

			rec = doRequest("POST", "/api/pull-requests", map[string]any{
				"title":              "PR for other",
				"object_type":        "global_values",
				"global_values_name": "other_values",
				"proposed_payload":   `{"key":"new"}`,
			}, bobID, "bob")
			Expect(rec.Code).To(Equal(http.StatusCreated))
		})

		It("returns all PRs without filter", func() {
			rec := doRequest("GET", "/api/pull-requests", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(2))
		})

		It("filters by global_values_name", func() {
			rec := doRequest("GET", "/api/pull-requests?global_values_name=test_db_values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(1))
			items := body["items"].([]any)
			pr := items[0].(map[string]any)
			Expect(pr["title"]).To(Equal("PR for test_db"))
		})
	})
})
