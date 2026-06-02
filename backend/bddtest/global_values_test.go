package bddtest

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Global Values Groups", func() {
	var aliceID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
	})

	Context("creating a global values group", func() {
		It("returns 201 with v1 and the values map for the doc example", func() {
			rec := doRequest("POST", "/api/global-values-groups", map[string]any{
				"name": "test_db_values",
				"values": map[string]any{
					"test_db_values": map[string]any{
						"host":     "test-db.internal",
						"port":     5432,
						"username": "app",
						"password": "s3cret",
					},
				},
				"commit_message": "Initial DB values",
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			group := body["group"].(map[string]any)
			Expect(group["name"]).To(Equal("test_db_values"))

			latest := body["latest_version"].(map[string]any)
			Expect(latest["version_id"]).To(BeEquivalentTo(1))
			values := latest["values"].(map[string]any)
			entry := values["test_db_values"].(map[string]any)
			Expect(entry["host"]).To(Equal("test-db.internal"))
			Expect(entry["port"]).To(BeEquivalentTo(5432))
		})

		It("rejects nested objects (flat-only constraint)", func() {
			rec := doRequest("POST", "/api/global-values-groups", map[string]any{
				"name": "bad_values",
				"values": map[string]any{
					"bad_values": map[string]any{
						"nested": map[string]any{"a": 1},
					},
				},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("accepts a list of strings", func() {
			rec := doRequest("POST", "/api/global-values-groups", map[string]any{
				"name": "net_values",
				"values": map[string]any{
					"net_values": map[string]any{
						"hosts": []string{"a.internal", "b.internal"},
					},
				},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusCreated))
		})

		It("rejects a list containing non-strings", func() {
			rec := doRequest("POST", "/api/global-values-groups", map[string]any{
				"name": "bad_values",
				"values": map[string]any{
					"bad_values": map[string]any{
						"ports": []any{80, 443},
					},
				},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("rejects duplicate group names with 409", func() {
			createGlobalValues(aliceID, "alice", "test_db_values", map[string]any{"host": "db.internal"})

			rec := doRequest("POST", "/api/global-values-groups", map[string]any{
				"name": "test_db_values",
				"values": map[string]any{
					"test_db_values": map[string]any{"host": "other"},
				},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusConflict))
		})

		It("rejects missing name with 400", func() {
			rec := doRequest("POST", "/api/global-values-groups", map[string]any{
				"values": map[string]any{"x": map[string]any{"host": "db"}},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("rejects missing values map with 400", func() {
			rec := doRequest("POST", "/api/global-values-groups", map[string]any{
				"name": "test",
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("appending group versions", func() {
		BeforeEach(func() {
			createGlobalValues(aliceID, "alice", "test_db_values", map[string]any{
				"host": "test-db.internal",
				"port": 5432,
			})
			seedGlobalValuesPermission(aliceID, "test_db_values")
		})

		It("creates version 2 with updated payload", func() {
			rec := doRequest("POST", "/api/global-values-groups/test_db_values/versions", map[string]any{
				"values": map[string]any{
					"test_db_values": map[string]any{
						"host": "new-db.internal",
						"port": 5433,
					},
				},
				"commit_message": "Update DB host",
			}, aliceID, "alice")

			Expect(rec.Code).To(Equal(http.StatusCreated))
			body := decode[map[string]any](rec)
			Expect(body["version_id"]).To(BeEquivalentTo(2))
			values := body["values"].(map[string]any)
			entry := values["test_db_values"].(map[string]any)
			Expect(entry["host"]).To(Equal("new-db.internal"))
		})

		It("preserves version 1 immutably", func() {
			doRequest("POST", "/api/global-values-groups/test_db_values/versions", map[string]any{
				"values": map[string]any{
					"test_db_values": map[string]any{"host": "new-db.internal", "port": 5433},
				},
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/global-values-groups/test_db_values/versions/1", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			values := body["values"].(map[string]any)
			entry := values["test_db_values"].(map[string]any)
			Expect(entry["host"]).To(Equal("test-db.internal"))
		})

		It("returns the latest version by default", func() {
			doRequest("POST", "/api/global-values-groups/test_db_values/versions", map[string]any{
				"values": map[string]any{
					"test_db_values": map[string]any{"host": "new-db.internal", "port": 5433},
				},
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/global-values-groups/test_db_values", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			latest := body["latest_version"].(map[string]any)
			Expect(latest["version_id"]).To(BeEquivalentTo(2))
		})

		It("lists all group versions in descending order", func() {
			doRequest("POST", "/api/global-values-groups/test_db_values/versions", map[string]any{
				"values": map[string]any{"test_db_values": map[string]any{"host": "v2", "port": 1}},
			}, aliceID, "alice")
			doRequest("POST", "/api/global-values-groups/test_db_values/versions", map[string]any{
				"values": map[string]any{"test_db_values": map[string]any{"host": "v3", "port": 2}},
			}, aliceID, "alice")

			rec := doRequest("GET", "/api/global-values-groups/test_db_values/versions", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(3))

			items := body["items"].([]any)
			first := items[0].(map[string]any)
			Expect(first["version_id"]).To(BeEquivalentTo(3))
		})

		It("rejects nested objects on version append", func() {
			rec := doRequest("POST", "/api/global-values-groups/test_db_values/versions", map[string]any{
				"values": map[string]any{
					"test_db_values": map[string]any{"nested": map[string]any{"a": 1}},
				},
			}, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("listing groups", func() {
		It("returns each group with its latest metadata", func() {
			createGlobalValues(aliceID, "alice", "test_db_values", map[string]any{"host": "db1"})
			createGlobalValues(aliceID, "alice", "shared_secrets", map[string]any{"api_key": "abc"})

			rec := doRequest("GET", "/api/global-values-groups", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(2))
		})

		It("returns empty list when none exist", func() {
			rec := doRequest("GET", "/api/global-values-groups", nil, aliceID, "alice")
			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["count"]).To(BeEquivalentTo(0))
		})
	})
})
