package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/brian/config-generation/backend/middleware"
	"github.com/brian/config-generation/backend/models"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

const (
	defaultSessionCookie = "configgen_session"
	csrfCookieName       = "configgen_csrf"
	oidcStateCookieName  = "configgen_oidc_state"
	oidcNonceCookieName  = "configgen_oidc_nonce"
	oidcReturnCookieName = "configgen_oidc_return_to"
)

type AuthConfig struct {
	JWTSecret               []byte
	OIDCEnabled             bool
	OIDCIssuerURL           string
	OIDCClientID            string
	OIDCClientSecret        string
	OIDCRedirectURL         string
	OIDCBrowserAuthURL      string
	OIDCScopes              []string
	OIDCProviderName        string
	OIDCSuperuserEmails     []string
	OIDCAllowedEmailDomains []string
	SessionCookieName       string
	SessionCookieSecure     bool
	SessionSameSite         http.SameSite
	PasswordLogin           bool
	Registration            bool
}

type AuthHandler struct {
	DB     *sql.DB
	Config AuthConfig

	oidcMu      sync.Mutex
	oauthConfig *oauth2.Config
	verifier    *oidc.IDTokenVerifier
}

type oidcUserClaims struct {
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
}

func DefaultAuthConfig(jwtSecret []byte) AuthConfig {
	return AuthConfig{
		JWTSecret:           jwtSecret,
		OIDCScopes:          []string{oidc.ScopeOpenID, "email", "profile"},
		OIDCProviderName:    "SSO",
		SessionCookieName:   defaultSessionCookie,
		SessionCookieSecure: true,
		SessionSameSite:     http.SameSiteLaxMode,
		PasswordLogin:       true,
		Registration:        true,
	}
}

func AuthConfigFromEnv(jwtSecret []byte) AuthConfig {
	cfg := DefaultAuthConfig(jwtSecret)
	cfg.OIDCEnabled = envBool("OIDC_ENABLED", false)
	cfg.OIDCIssuerURL = strings.TrimSpace(os.Getenv("OIDC_ISSUER_URL"))
	cfg.OIDCClientID = strings.TrimSpace(os.Getenv("OIDC_CLIENT_ID"))
	cfg.OIDCClientSecret = os.Getenv("OIDC_CLIENT_SECRET")
	cfg.OIDCRedirectURL = strings.TrimSpace(os.Getenv("OIDC_REDIRECT_URL"))
	cfg.OIDCBrowserAuthURL = strings.TrimSpace(os.Getenv("OIDC_BROWSER_AUTH_URL"))
	cfg.OIDCProviderName = envString("OIDC_PROVIDER_NAME", cfg.OIDCProviderName)
	cfg.OIDCSuperuserEmails = envList("OIDC_SUPERUSER_EMAILS")
	cfg.OIDCAllowedEmailDomains = envList("OIDC_ALLOWED_EMAIL_DOMAINS")
	cfg.SessionCookieName = envString("SESSION_COOKIE_NAME", cfg.SessionCookieName)
	cfg.SessionCookieSecure = envBool("SESSION_COOKIE_SECURE", cfg.SessionCookieSecure)
	cfg.SessionSameSite = parseSameSite(envString("SESSION_COOKIE_SAMESITE", "Lax"))
	cfg.PasswordLogin = envBool("PASSWORD_LOGIN_ENABLED", true)
	cfg.Registration = envBool("REGISTRATION_ENABLED", true)
	if scopes := strings.Fields(os.Getenv("OIDC_SCOPES")); len(scopes) > 0 {
		cfg.OIDCScopes = scopes
	}
	return cfg
}

func (h *AuthHandler) ConfigResponse(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, models.AuthConfigResponse{
		OIDCEnabled:          h.Config.OIDCEnabled,
		OIDCProviderName:     h.Config.OIDCProviderName,
		PasswordLoginEnabled: h.Config.PasswordLogin,
		RegistrationEnabled:  h.Config.Registration,
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if !h.Config.Registration {
		writeError(w, http.StatusNotFound, "registration is disabled", "not_found")
		return
	}

	var req models.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required", "bad_request")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters", "bad_request")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password", "internal")
		return
	}

	var user models.User
	err = h.DB.QueryRowContext(r.Context(),
		`INSERT INTO users (username, display_name, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id, username, display_name, created_at`,
		req.Username, req.DisplayName, string(hash),
	).Scan(&user.ID, &user.Username, &user.DisplayName, &user.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "username already taken", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user", "internal")
		return
	}

	h.writeAuthResponse(w, http.StatusCreated, user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.Config.PasswordLogin {
		writeError(w, http.StatusNotFound, "password login is disabled", "not_found")
		return
	}

	var req models.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	var user models.User
	var passwordHash string
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT id, username, display_name, created_at, password_hash
		 FROM users WHERE username = $1`,
		req.Username,
	).Scan(&user.ID, &user.Username, &user.DisplayName, &user.CreatedAt, &passwordHash)
	if err != nil || passwordHash == "" {
		writeError(w, http.StatusUnauthorized, "invalid username or password", "unauthorized")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password", "unauthorized")
		return
	}

	h.writeAuthResponse(w, http.StatusOK, user)
}

func (h *AuthHandler) Session(w http.ResponseWriter, r *http.Request) {
	var req models.SessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required", "bad_request")
		return
	}

	user, err := middleware.ParseToken(req.Token, h.Config.JWTSecret, middleware.AuthMethodBearer)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid token", "unauthorized")
		return
	}

	dbUser, err := h.loadUserByID(r.Context(), user.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid token", "unauthorized")
		return
	}

	h.setSessionCookies(w, req.Token)
	writeJSON(w, http.StatusOK, models.AuthResponse{Token: req.Token, User: dbUser})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	authUser, err := middleware.AuthenticateRequest(r, h.Config.JWTSecret, h.Config.SessionCookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	me, err := h.loadMeByID(r.Context(), authUser.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	writeJSON(w, http.StatusOK, me)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie(h.Config.SessionCookieName); err == nil {
		if !validCSRF(r) {
			writeError(w, http.StatusForbidden, "invalid CSRF token", "csrf")
			return
		}
	}
	h.clearCookie(w, h.Config.SessionCookieName)
	h.clearCSRFCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	if !h.Config.OIDCEnabled {
		writeError(w, http.StatusNotFound, "OIDC is disabled", "not_found")
		return
	}

	oauthConfig, _, err := h.ensureOIDC(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize OIDC", "oidc_unavailable")
		return
	}

	state, err := randomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create login state", "internal")
		return
	}
	nonce, err := randomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create login nonce", "internal")
		return
	}
	returnTo := safeReturnTo(r.URL.Query().Get("return_to"))

	h.setCookie(w, oidcStateCookieName, state, 10*time.Minute)
	h.setCookie(w, oidcNonceCookieName, nonce, 10*time.Minute)
	h.setCookie(w, oidcReturnCookieName, returnTo, 10*time.Minute)

	http.Redirect(w, r, oauthConfig.AuthCodeURL(state, oidc.Nonce(nonce)), http.StatusFound)
}

func (h *AuthHandler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	if !h.Config.OIDCEnabled {
		writeError(w, http.StatusNotFound, "OIDC is disabled", "not_found")
		return
	}

	stateCookie, err := r.Cookie(oidcStateCookieName)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		writeError(w, http.StatusUnauthorized, "invalid OIDC state", "unauthorized")
		return
	}

	nonceCookie, err := r.Cookie(oidcNonceCookieName)
	if err != nil || nonceCookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "missing OIDC nonce", "unauthorized")
		return
	}

	oauthConfig, verifier, err := h.ensureOIDC(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize OIDC", "oidc_unavailable")
		return
	}

	oauthToken, err := oauthConfig.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "failed to exchange OIDC code", "unauthorized")
		return
	}

	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		writeError(w, http.StatusUnauthorized, "OIDC response missing ID token", "unauthorized")
		return
	}

	idToken, err := verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid OIDC ID token", "unauthorized")
		return
	}
	if idToken.Nonce != nonceCookie.Value {
		writeError(w, http.StatusUnauthorized, "invalid OIDC nonce", "unauthorized")
		return
	}

	var claims oidcUserClaims
	if err := idToken.Claims(&claims); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid OIDC claims", "unauthorized")
		return
	}

	if !h.isAllowedOIDCEmail(claims) {
		writeError(w, http.StatusForbidden, "email domain not permitted", "forbidden")
		return
	}

	user, err := h.findOrCreateOIDCUser(r.Context(), idToken.Issuer, idToken.Subject, claims)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to provision user", "internal")
		return
	}

	token, err := h.generateToken(user.ID, user.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token", "internal")
		return
	}
	h.setSessionCookies(w, token)
	h.clearCookie(w, oidcStateCookieName)
	h.clearCookie(w, oidcNonceCookieName)
	h.clearCookie(w, oidcReturnCookieName)

	returnTo := "/projects"
	if cookie, err := r.Cookie(oidcReturnCookieName); err == nil {
		returnTo = safeReturnTo(cookie.Value)
	}
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func (h *AuthHandler) writeAuthResponse(w http.ResponseWriter, status int, user models.User) {
	token, err := h.generateToken(user.ID, user.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token", "internal")
		return
	}

	h.setSessionCookies(w, token)
	writeJSON(w, status, models.AuthResponse{Token: token, User: user})
}

func (h *AuthHandler) generateToken(userID int64, username string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  float64(userID),
		"username": username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.Config.JWTSecret)
}

func (h *AuthHandler) loadUserByID(ctx context.Context, userID int64) (models.User, error) {
	var user models.User
	err := h.DB.QueryRowContext(ctx,
		`SELECT id, username, display_name, created_at FROM users WHERE id = $1`,
		userID,
	).Scan(&user.ID, &user.Username, &user.DisplayName, &user.CreatedAt)
	return user, err
}

// loadMeByID is like loadUserByID but additionally includes the superuser flag.
// It is intended only for /api/auth/me; other endpoints must not leak this flag.
func (h *AuthHandler) loadMeByID(ctx context.Context, userID int64) (models.MeResponse, error) {
	var me models.MeResponse
	err := h.DB.QueryRowContext(ctx,
		`SELECT id, username, display_name, created_at, superuser FROM users WHERE id = $1`,
		userID,
	).Scan(&me.ID, &me.Username, &me.DisplayName, &me.CreatedAt, &me.Superuser)
	return me, err
}

func (h *AuthHandler) ensureOIDC(ctx context.Context) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	if h.oauthConfig != nil && h.verifier != nil {
		return h.oauthConfig, h.verifier, nil
	}

	h.oidcMu.Lock()
	defer h.oidcMu.Unlock()
	if h.oauthConfig != nil && h.verifier != nil {
		return h.oauthConfig, h.verifier, nil
	}
	if h.Config.OIDCIssuerURL == "" || h.Config.OIDCClientID == "" || h.Config.OIDCClientSecret == "" || h.Config.OIDCRedirectURL == "" {
		return nil, nil, errors.New("OIDC config is incomplete")
	}

	provider, err := oidc.NewProvider(ctx, h.Config.OIDCIssuerURL)
	if err != nil {
		return nil, nil, err
	}
	h.oauthConfig = &oauth2.Config{
		ClientID:     h.Config.OIDCClientID,
		ClientSecret: h.Config.OIDCClientSecret,
		RedirectURL:  h.Config.OIDCRedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       h.Config.OIDCScopes,
	}
	if h.Config.OIDCBrowserAuthURL != "" {
		h.oauthConfig.Endpoint.AuthURL = h.Config.OIDCBrowserAuthURL
	}
	h.verifier = provider.Verifier(&oidc.Config{ClientID: h.Config.OIDCClientID})
	return h.oauthConfig, h.verifier, nil
}

func (h *AuthHandler) findOrCreateOIDCUser(ctx context.Context, issuer, subject string, claims oidcUserClaims) (models.User, error) {
	user, err := h.loadOIDCUserByIdentity(ctx, issuer, subject)
	if err == nil {
		return user, h.promoteOIDCSuperuser(ctx, user.ID, claims)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return user, err
	}

	username := oidcUsername(issuer, subject, claims)
	displayName := oidcDisplayName(username, claims)
	email := strings.TrimSpace(strings.ToLower(claims.Email))
	superuser := h.isOIDCSuperuser(claims)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return user, err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx,
		`INSERT INTO users (username, display_name, password_hash, superuser)
		 VALUES ($1, $2, '', $3)
		 RETURNING id, username, display_name, created_at`,
		username, displayName, superuser,
	).Scan(&user.ID, &user.Username, &user.DisplayName, &user.CreatedAt)
	if err != nil && isUniqueViolation(err) {
		username = fallbackOIDCUsername(issuer, subject)
		displayName = oidcDisplayName(username, claims)
		err = tx.QueryRowContext(ctx,
			`INSERT INTO users (username, display_name, password_hash, superuser)
			 VALUES ($1, $2, '', $3)
			 RETURNING id, username, display_name, created_at`,
			username, displayName, superuser,
		).Scan(&user.ID, &user.Username, &user.DisplayName, &user.CreatedAt)
	}
	if err != nil {
		return user, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_identities (user_id, provider, issuer, subject, email)
		VALUES ($1, $2, $3, $4, $5)
	`, user.ID, h.Config.OIDCProviderName, issuer, subject, nullableString(email))
	if err != nil {
		if isUniqueViolation(err) {
			tx.Rollback()
			return h.loadOIDCUserByIdentity(ctx, issuer, subject)
		}
		return user, err
	}

	return user, tx.Commit()
}

func (h *AuthHandler) promoteOIDCSuperuser(ctx context.Context, userID int64, claims oidcUserClaims) error {
	if !h.isOIDCSuperuser(claims) {
		return nil
	}
	_, err := h.DB.ExecContext(ctx, `UPDATE users SET superuser = true WHERE id = $1`, userID)
	return err
}

func (h *AuthHandler) isOIDCSuperuser(claims oidcUserClaims) bool {
	if !claims.EmailVerified {
		return false
	}
	email := strings.TrimSpace(strings.ToLower(claims.Email))
	if email == "" {
		return false
	}
	for _, allowed := range h.Config.OIDCSuperuserEmails {
		if email == strings.TrimSpace(strings.ToLower(allowed)) {
			return true
		}
	}
	return false
}

// isAllowedOIDCEmail returns true when the claims' verified email matches one
// of OIDCAllowedEmailDomains. When the allowlist is empty, all verified emails
// are accepted. Unverified emails are always rejected.
func (h *AuthHandler) isAllowedOIDCEmail(claims oidcUserClaims) bool {
	if len(h.Config.OIDCAllowedEmailDomains) == 0 {
		return true
	}
	if !claims.EmailVerified {
		return false
	}
	email := strings.TrimSpace(strings.ToLower(claims.Email))
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return false
	}
	domain := email[at+1:]
	for _, allowed := range h.Config.OIDCAllowedEmailDomains {
		if domain == strings.TrimSpace(strings.ToLower(allowed)) {
			return true
		}
	}
	return false
}

func (h *AuthHandler) loadOIDCUserByIdentity(ctx context.Context, issuer, subject string) (models.User, error) {
	var user models.User
	err := h.DB.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.display_name, u.created_at
		FROM user_identities ui
		JOIN users u ON u.id = ui.user_id
		WHERE ui.issuer = $1 AND ui.subject = $2
	`, issuer, subject).Scan(&user.ID, &user.Username, &user.DisplayName, &user.CreatedAt)
	return user, err
}

func (h *AuthHandler) setSessionCookies(w http.ResponseWriter, token string) {
	h.setCookie(w, h.Config.SessionCookieName, token, 24*time.Hour)
	if csrf, err := randomToken(); err == nil {
		h.setCSRFCookie(w, csrf, 24*time.Hour)
	}
}

// setCookie writes an HttpOnly cookie. Use this for all session/state tokens.
// The CSRF double-submit cookie must NOT be HttpOnly — use setCSRFCookie.
func (h *AuthHandler) setCookie(w http.ResponseWriter, name, value string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		Expires:  time.Now().Add(maxAge),
		HttpOnly: true,
		// Secure is driven by SESSION_COOKIE_SECURE env var: true in production
		// (HTTPS), false only for local docker-compose / Kind dev over plain HTTP.
		Secure:   h.Config.SessionCookieSecure, // NOSONAR – configurable per environment
		SameSite: h.Config.SessionSameSite,
	})
}

// setCSRFCookie writes the CSRF double-submit cookie. It is intentionally
// NOT HttpOnly because the frontend reads its value and echoes it back in the
// X-CSRF-Token header on every state-changing request (validated by
// middleware/csrf.go). The cookie's value is a 256-bit random token that has
// no authentication meaning on its own — forging it does not grant access.
// SONAR_OFF: CSRF double-submit cookie (see middleware/csrf.go)
func (h *AuthHandler) setCSRFCookie(w http.ResponseWriter, value string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		Expires:  time.Now().Add(maxAge),
		HttpOnly: false, // NOSONAR – double-submit pattern requires JS access
		Secure:   h.Config.SessionCookieSecure,
		SameSite: h.Config.SessionSameSite,
	})
}

// SONAR_ON

// clearCookie expires an HttpOnly cookie.
func (h *AuthHandler) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		// Must mirror setCookie so browsers actually clear the cookie in production HTTPS.
		Secure:   h.Config.SessionCookieSecure, // NOSONAR – configurable per environment
		SameSite: h.Config.SessionSameSite,
	})
}

// clearCSRFCookie expires the CSRF cookie (non-HttpOnly counterpart).
// SONAR_OFF: CSRF double-submit cookie (see middleware/csrf.go)
func (h *AuthHandler) clearCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: false, // NOSONAR – must mirror setCSRFCookie
		Secure:   h.Config.SessionCookieSecure,
		SameSite: h.Config.SessionSameSite,
	})
}

// SONAR_ON

func validCSRF(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	return r.Header.Get(middleware.CSRFHeaderName) == cookie.Value
}

func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func safeReturnTo(raw string) string {
	if raw == "" {
		return "/projects"
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return "/projects"
	}
	return raw
}

func oidcUsername(issuer, subject string, claims oidcUserClaims) string {
	email := strings.TrimSpace(strings.ToLower(claims.Email))
	if claims.EmailVerified && email != "" {
		return email
	}
	return fallbackOIDCUsername(issuer, subject)
}

func oidcDisplayName(username string, claims oidcUserClaims) *string {
	for _, candidate := range []string{claims.Name, claims.PreferredUsername, claims.Email, username} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return &candidate
		}
	}
	return nil
}

func fallbackOIDCUsername(issuer, subject string) string {
	sum := sha256.Sum256([]byte(issuer + "\x00" + subject))
	return "oidc_" + hex.EncodeToString(sum[:])[:16]
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envList(name string) []string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			values = append(values, part)
		}
	}
	return values
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax", "":
		return http.SameSiteLaxMode
	default:
		fmt.Printf("unknown SESSION_COOKIE_SAMESITE %q, using Lax\n", value)
		return http.SameSiteLaxMode
	}
}
