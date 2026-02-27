package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/cyphergurke/audistro-catalog/internal/httpapi/middleware"
)

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type APIErrorResponse struct {
	Error APIError `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code string, message string) {
	writeJSON(w, status, APIErrorResponse{
		Error: APIError{
			Code:      code,
			Message:   message,
			RequestID: middleware.GetRequestID(r.Context()),
		},
	})
}
