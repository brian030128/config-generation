package bddtest

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func usernamesInList(rec *httptest.ResponseRecorder) []string {
	GinkgoHelper()
	body := decode[map[string]any](rec)
	names := []string{}
	for _, item := range body["items"].([]any) {
		names = append(names, item.(map[string]any)["username"].(string))
	}
	return names
}

var _ = Describe("User directory search", func() {
	var aliceID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		seedUser("bob", "Bob Jones")
		seedUser("carol", "Carol Alice Danvers")
	})

	It("finds users by username substring", func() {
		rec := doRequest("GET", "/api/users?search=bo", nil, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(usernamesInList(rec)).To(ConsistOf("bob"))
	})

	It("matches the display name case-insensitively", func() {
		// "alice" appears in alice's username and carol's display name.
		rec := doRequest("GET", "/api/users?search=ALICE", nil, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(usernamesInList(rec)).To(ConsistOf("alice", "carol"))
	})

	It("returns no matches for an unknown term", func() {
		rec := doRequest("GET", "/api/users?search=zzz", nil, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))
		body := decode[map[string]any](rec)
		Expect(body["count"]).To(BeEquivalentTo(0))
	})

	It("returns all users when the search is empty", func() {
		rec := doRequest("GET", "/api/users", nil, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(usernamesInList(rec)).To(ConsistOf("alice", "bob", "carol"))
	})
})
