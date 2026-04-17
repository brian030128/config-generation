package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/brian/config-generation/backend/models"
)

func writeError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(models.ErrorResponse{Error: message, Code: code})
}
