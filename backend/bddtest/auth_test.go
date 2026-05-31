package bddtest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Authentication sessions", func() {
	BeforeEach(func() {
		truncateAll()
	})

	It("exposes auth feature configuration", func() {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/config", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusOK))
		body := decode[map[string]any](rec)
		Expect(body["oidc_enabled"]).To(Equal(false))
		Expect(body["password_login_enabled"]).To(Equal(true))
		Expect(body["registration_enabled"]).To(Equal(true))
	})

	It("migrates a bearer token into session and CSRF cookies", func() {
		userID := seedUser("alice", "Alice")
		token := mintToken(userID, "alice")
		payload, err := json.Marshal(map[string]string{"token": token})
		Expect(err).NotTo(HaveOccurred())

		req := httptest.NewRequest(http.MethodPost, "/api/auth/session", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(rec.Result().Cookies()).To(ContainElement(WithTransform(func(c *http.Cookie) string {
			return c.Name
		}, Equal("configgen_session"))))
		Expect(rec.Result().Cookies()).To(ContainElement(WithTransform(func(c *http.Cookie) string {
			return c.Name
		}, Equal("configgen_csrf"))))
	})

	It("requires CSRF headers for unsafe cookie-authenticated requests", func() {
		userID := seedUser("alice", "Alice")
		seedSystemRole(userID)
		token := mintToken(userID, "alice")

		payload, err := json.Marshal(map[string]string{"name": "billing"})
		Expect(err).NotTo(HaveOccurred())

		req := httptest.NewRequest(http.MethodPost, "/api/projects/", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "configgen_session", Value: token})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusForbidden))

		req = httptest.NewRequest(http.MethodPost, "/api/projects/", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", "csrf-token")
		req.AddCookie(&http.Cookie{Name: "configgen_session", Value: token})
		req.AddCookie(&http.Cookie{Name: "configgen_csrf", Value: "csrf-token"})
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusCreated))
	})

	It("keeps bearer-token requests exempt from CSRF checks", func() {
		userID := seedUser("alice", "Alice")
		seedSystemRole(userID)

		rec := doRequest(http.MethodPost, "/api/projects/", map[string]string{
			"name": "billing",
		}, userID, "alice")

		Expect(rec.Code).To(Equal(http.StatusCreated))
	})

	Describe("GET /api/auth/me", func() {
		It("returns superuser=false for a regular user", func() {
			userID := seedUser("alice", "Alice")

			rec := doRequest(http.MethodGet, "/api/auth/me", nil, userID, "alice")

			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["username"]).To(Equal("alice"))
			Expect(body["superuser"]).To(Equal(false))
		})

		It("returns superuser=true for a superuser", func() {
			userID := seedSuperuser("root", "Root")

			rec := doRequest(http.MethodGet, "/api/auth/me", nil, userID, "root")

			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			Expect(body["username"]).To(Equal("root"))
			Expect(body["superuser"]).To(Equal(true))
		})

		It("does not leak the superuser flag via the users directory search", func() {
			seedSuperuser("root", "Root")
			callerID := seedUser("alice", "Alice")

			rec := doRequest(http.MethodGet, "/api/users?search=root", nil, callerID, "alice")

			Expect(rec.Code).To(Equal(http.StatusOK))
			body := decode[map[string]any](rec)
			items, ok := body["items"].([]any)
			Expect(ok).To(BeTrue())
			Expect(items).NotTo(BeEmpty())
			for _, item := range items {
				entry, ok := item.(map[string]any)
				Expect(ok).To(BeTrue())
				_, hasSU := entry["superuser"]
				Expect(hasSU).To(BeFalse(), "user directory response must not include superuser flag")
			}
		})
	})
})
