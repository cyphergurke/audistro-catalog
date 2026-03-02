package middleware

import (
	"net/http"
)

type ReadOnlyConfig struct {
	Enabled bool
}

func ReadOnly(cfg ReadOnlyConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if !cfg.Enabled {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"read_only"}`))
				return
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}
