package handlers

import (
	"net"
	"net/http"
	"strings"
	"time"

	"audistro-catalog/internal/ratelimit"
)

func withRateLimit(next http.HandlerFunc, limiter *ratelimit.Limiter) http.HandlerFunc {
	if limiter == nil {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(clientIPFromRemoteAddr(r.RemoteAddr), time.Now()) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate_limited"}`))
			return
		}
		next(w, r)
	}
}

func clientIPFromRemoteAddr(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return nil
	}
	return net.ParseIP(host)
}
