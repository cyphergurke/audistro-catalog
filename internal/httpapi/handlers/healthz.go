package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/cyphergurke/audistro-catalog/internal/httpapi/middleware"
)

// HealthzResponse is the typed health check response payload.
type HealthzResponse struct {
	Status    string `json:"status"`
	RequestID string `json:"request_id"`
}

// Healthz responds with the service health status.
func Healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	response := HealthzResponse{
		Status:    "ok",
		RequestID: middleware.GetRequestID(r.Context()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
