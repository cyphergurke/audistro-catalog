package httpapi

import (
	"log"
	"net/http"

	"audistro-catalog/internal/config"
	"audistro-catalog/internal/httpapi/handlers"
	"audistro-catalog/internal/httpapi/middleware"
)

// Server wraps the HTTP server.
type Server struct {
	httpServer *http.Server
}

// NewServer constructs the HTTP server with middleware and routes.
func NewServer(cfg config.Config, logger *log.Logger, deps handlers.Dependencies) *Server {
	routes := handlers.NewRouter(deps)

	var handler http.Handler = routes
	handler = middleware.Recover(logger)(handler)
	handler = middleware.AccessLog(logger)(handler)
	handler = openAPIValidationMiddleware(handler)
	handler = middleware.RequestID(handler)

	return &Server{
		httpServer: &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: handler,
		},
	}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}
