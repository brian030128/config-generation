package bddtest

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Global Values", func() {
	var aliceID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
	})

	Context("creating a global values entry", func() {
		It("returns 201 with version_id=1 using the doc example test_db_values", func() {
			rec := doRequest("POST", "/api/global-values", map[string]any{
				"name": "test_db_values",
				"payload": map[string]any{
					"host":     "test-db.internal",
					"port":     5432,
					"username": "app",
					"password": "s3cret",
				},
				"commit_message": "Initial DB values",
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["name"]).To(Equal("test_db_values"))
			Expect(body["version_id"]).To(BeEquivalentTo(1))

			payload := body["payload"].(map[string]any)
			Expect(payload["host"]).To(Equal("test-db.internal"))
			Expect(payload["port"]).To(BeEquivalentTo(5432))
			Expect(payload["username"]).To(Equal("app"))
			Expect(payload["password"]).To(Equal("s3cret"))
		})

		It("rejects nested objects (flat-only constraint)", func() {
			rec := doRequest("POST", "/api/global-values", map[string]any{
				"name": "bad_values",
				"payload": map[string]any{
					"nested": map[string]any{"a": 1},
				},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("rejects arrays in payload", func() {
			rec := doRequest("POST", "/api/global-values", map[string]any{
				"name": "bad_values",
				"payload": map[string]any{
					"list": []string{"a", "b"},
				},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("rejects duplicate names with 409", func() {
			createGlobalValues(aliceID, "alice", "test_db_values", map[string]any{"host": "db.internal"})

			rec := doRequest("POST", "/api/global-values", map[string]any{
				"name":    "test_db_values",
				"payload": map[string]any{"host": "other"},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusConflict))
		})

		It("rejects missing name with 400", func() {
			rec := doRequest("POST", "/api/global-values", map[string]any{
				"payload": map[string]any{"host": "db"},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("rejects missing payload with 400", func() {
			rec := doRequest("POST", "/api/global-values", map[string]any{
				"name": "test",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("appending versions", func() {
		BeforeEach(func() {
			createGlobalValues(aliceID, "alice", "test_db_values", map[string]any{
				"host": "test-db.internal",
				"port": 5432,
			})
			seedGlobalValuesPermission(aliceID, "test_db_values")
		})

		It("creates version 2 with updated payload", func() {
			rec := doRequest("POST", "/api/global-values/test_db_values/versions", map[string]any{
				"payload": map[string]any{
					"host": "new-db.internal",
					"port": 5433,
				},
				"commit_message": "Update DB host",
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(2))
			payload := body["payload"].(map[string]any)
			Expect(payload["host"]).To(Equal("new-db.internal"))
		})

		It("preserves version 1 immutably", func() {
			doRequest("POST", "/api/global-values/test_db_values/versions", map[string]any{
				"payload": map[string]any{"host": "new-db.internal", "port": 5433},
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/global-values/test_db_values/versions/1", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			payload := body["payload"].(map[string]any)
			Expect(payload["host"]).To(Equal("test-db.internal"))
		})

		It("returns the latest version by default", func() {
			doRequest("POST", "/api/global-values/test_db_values/versions", map[string]any{
				"payload": map[string]any{"host": "new-db.internal", "port": 5433},
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/global-values/test_db_values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(2))
		})

		It("lists all versions in descending order", func() {
			doRequest("POST", "/api/global-values/test_db_values/versions", map[string]any{
				"payload": map[string]any{"host": "v2", "port": 1},
			}, aliceID, "alice")
			doRequest("POST", "/api/global-values/test_db_values/versions", map[string]any{
				"payload": map[string]any{"host": "v3", "port": 2},
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/global-values/test_db_values/versions", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(3))

			items := body["items"].([]any)
			first := items[0].(map[string]any)
			Expect(first["version_id"]).To(BeEquivalentTo(3))
		})

		It("rejects nested objects on version append", func() {
			rec := doRequest("POST", "/api/global-values/test_db_values/versions", map[string]any{
				"payload": map[string]any{"nested": map[string]any{"a": 1}},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("listing global values", func() {
		It("returns the latest version of each entry", func() {
			createGlobalValues(aliceID, "alice", "test_db_values", map[string]any{"host": "db1"})
			createGlobalValues(aliceID, "alice", "shared_secrets", map[string]any{"api_key": "abc"})

			rec := doRequest("GET", "/api/global-values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(2))
		})

		It("returns empty list when none exist", func() {
			rec := doRequest("GET", "/api/global-values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(0))
		})
	})
})
