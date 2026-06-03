package bddtest

import (
	"context"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// insertProjectPR inserts a pull request directly against a project, bypassing
// the workspace API so the test can set up sibling PRs without granting the
// author write permissions. Returns the new PR id.
func insertProjectPR(projectName string, authorID int64, status string) int64 {
	GinkgoHelper()
	var projectID int64
	err := testDB.QueryRowContext(context.Background(),
		`SELECT id FROM projects WHERE name = $1`, projectName).Scan(&projectID)
	Expect(err).NotTo(HaveOccurred())

	var prID int64
	err = testDB.QueryRowContext(context.Background(), `
		INSERT INTO pull_requests (project_id, author_id, title, status)
		VALUES ($1, $2, $3, $4) RETURNING id
	`, projectID, authorID, "sibling", status).Scan(&prID)
	Expect(err).NotTo(HaveOccurred())
	return prID
}

func prStatus(readerID int64, readerName string, prID int64) map[string]any {
	GinkgoHelper()
	rec := doRequest("GET", fmt.Sprintf("/api/pull-requests/%d", prID), nil, readerID, readerName)
	Expect(rec.Code).To(Equal(http.StatusOK))
	return decode[map[string]any](rec)
}

var _ = Describe("Auto-close sibling project PRs on merge", func() {
	var aliceID, bobID int64

	BeforeEach(func() {
		truncateAll()
		aliceID = seedUser("alice", "Alice Smith")
		bobID = seedUser("bob", "Bob Jones")
		seedSystemRole(aliceID)
		createProject(aliceID, "alice", "my-service")
	})

	It("closes other open/draft PRs for the same project when one merges", func() {
		// Siblings authored by bob so they don't collide with alice's workspace
		// lookup (which is scoped per author).
		openSibling := insertProjectPR("my-service", bobID, "open")
		draftSibling := insertProjectPR("my-service", bobID, "draft")

		By("alice stages a change and merges her workspace PR")
		rec := doRequest("POST", "/api/workspace/my-service/environments",
			map[string]any{"name": "staging"}, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))
		submitApproveMerge(aliceID, "alice", "my-service")

		By("the sibling PRs/workspaces are now closed")
		openBody := prStatus(aliceID, "alice", openSibling)
		Expect(openBody["status"]).To(Equal("closed"))
		Expect(openBody["closed_at"]).NotTo(BeNil())

		draftBody := prStatus(aliceID, "alice", draftSibling)
		Expect(draftBody["status"]).To(Equal("closed"))
		Expect(draftBody["closed_at"]).NotTo(BeNil())
	})

	It("does not close PRs for a different project", func() {
		createProject(aliceID, "alice", "other-service")
		otherProjectPR := insertProjectPR("other-service", bobID, "open")

		By("alice merges a change in my-service")
		rec := doRequest("POST", "/api/workspace/my-service/environments",
			map[string]any{"name": "staging"}, aliceID, "alice")
		Expect(rec.Code).To(Equal(http.StatusOK))
		submitApproveMerge(aliceID, "alice", "my-service")

		By("the other project's PR is untouched")
		body := prStatus(aliceID, "alice", otherProjectPR)
		Expect(body["status"]).To(Equal("open"))
	})
})
