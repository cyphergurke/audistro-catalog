package middleware

import (
	"encoding/json"
	"log"
	"net/http"
)

type errorResponse struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id"`
}

// Recover converts handler panics into a typed JSON 500 response.
func Recover(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					requestID := GetRequestID(r.Context())
					logger.Printf("panic recovered request_id=%s err=%v", requestID, recovered)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)

					response := errorResponse{
						Error:     "internal_server_error",
						RequestID: requestID,
					}
					_ = json.NewEncoder(w).Encode(response)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
