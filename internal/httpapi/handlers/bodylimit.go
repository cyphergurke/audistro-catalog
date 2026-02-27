package handlers

import "net/http"

func withBodyLimit(next http.HandlerFunc, maxBytes int64) http.HandlerFunc {
	if maxBytes <= 0 {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next(w, r)
	}
}
