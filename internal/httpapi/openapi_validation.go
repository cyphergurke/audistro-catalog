package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"audistro-catalog/internal/apidocs"
	middlewarepkg "audistro-catalog/internal/httpapi/middleware"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
)

type validationAPIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type validationAPIErrorResponse struct {
	Error validationAPIError `json:"error"`
}

func openAPIValidationMiddleware(next http.Handler) http.Handler {
	spec, err := apidocs.LoadSpec()
	if err != nil {
		panic(err)
	}
	router, err := legacyrouter.NewRouter(spec)
	if err != nil {
		panic(err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !shouldValidateOpenAPIRequest(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		route, pathParams, err := router.FindRoute(r)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		if !requestContentTypeAllowed(route, r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(validationAPIErrorResponse{Error: validationAPIError{
				Code:      "invalid_request",
				Message:   "content type does not match OpenAPI contract",
				RequestID: middlewarepkg.GetRequestID(r.Context()),
			}})
			return
		}
		input := &openapi3filter.RequestValidationInput{
			Request:    r,
			PathParams: pathParams,
			Route:      route,
			Options: &openapi3filter.Options{
				AuthenticationFunc: func(context.Context, *openapi3filter.AuthenticationInput) error { return nil },
			},
		}
		if err := openapi3filter.ValidateRequest(r.Context(), input); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(validationAPIErrorResponse{Error: validationAPIError{
				Code:      "invalid_request",
				Message:   err.Error(),
				RequestID: middlewarepkg.GetRequestID(r.Context()),
			}})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func shouldValidateOpenAPIRequest(path string) bool {
	switch {
	case path == "/healthz":
		return false
	case path == apidocs.OpenAPIYAMLPath, path == apidocs.OpenAPIJSONPath, path == "/docs", path == "/docs/":
		return false
	case strings.HasPrefix(path, "/v1/"):
		return true
	default:
		return false
	}
}

func requestContentTypeAllowed(route *routers.Route, r *http.Request) bool {
	if route == nil || route.Operation == nil || route.Operation.RequestBody == nil || route.Operation.RequestBody.Value == nil {
		return true
	}
	contentTypes := route.Operation.RequestBody.Value.Content
	if len(contentTypes) == 0 {
		return true
	}
	contentType := strings.TrimSpace(strings.ToLower(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if contentType == "" {
		return false
	}
	_, ok := contentTypes[contentType]
	return ok
}
