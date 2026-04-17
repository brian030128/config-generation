package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/brian/config-generation/backend/models"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB        *sql.DB
	JWTSecret []byte
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
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

	token, err := h.generateToken(user.ID, user.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token", "internal")
		return
	}

	writeJSON(w, http.StatusCreated, models.AuthResponse{Token: token, User: user})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
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
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password", "unauthorized")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password", "unauthorized")
		return
	}

	token, err := h.generateToken(user.ID, user.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token", "internal")
		return
	}

	writeJSON(w, http.StatusOK, models.AuthResponse{Token: token, User: user})
}

func (h *AuthHandler) generateToken(userID int64, username string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  float64(userID),
		"username": username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.JWTSecret)
}
