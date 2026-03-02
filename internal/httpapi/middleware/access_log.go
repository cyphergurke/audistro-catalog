package middleware

import (
	"log"
	"net"
	"net/http"
	"time"
)

// AccessLog writes one structured-ish line per HTTP request.
func AccessLog(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &accessLogResponseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rw, r)

			logger.Printf(
				"http_request method=%s path=%s status=%d bytes=%d latency_ms=%d request_id=%s remote_ip=%s",
				r.Method,
				r.URL.Path,
				rw.status,
				rw.bytes,
				time.Since(start).Milliseconds(),
				GetRequestID(r.Context()),
				remoteIP(r.RemoteAddr),
			)
		})
	}
}

type accessLogResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *accessLogResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *accessLogResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
