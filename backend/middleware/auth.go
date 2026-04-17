package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserContextKey contextKey = "user"

// AuthUser represents the authenticated user extracted from the JWT.
type AuthUser struct {
	UserID   int64
	Username string
}

// JWTAuth returns middleware that validates the Bearer token and injects
// AuthUser into the request context.
func JWTAuth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "missing or malformed Authorization header", "unauthorized")
				return
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return secret, nil
			})
			if err != nil || !token.Valid {
				writeError(w, http.StatusUnauthorized, "invalid token", "unauthorized")
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				writeError(w, http.StatusUnauthorized, "invalid claims", "unauthorized")
				return
			}

			userIDFloat, _ := claims["user_id"].(float64)
			username, _ := claims["username"].(string)
			if userIDFloat == 0 || username == "" {
				writeError(w, http.StatusUnauthorized, "missing user claims", "unauthorized")
				return
			}

			user := AuthUser{
				UserID:   int64(userIDFloat),
				Username: username,
			}
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext extracts the authenticated user from the request context.
// Must only be called inside authenticated routes.
func UserFromContext(ctx context.Context) AuthUser {
	return ctx.Value(UserContextKey).(AuthUser)
}
