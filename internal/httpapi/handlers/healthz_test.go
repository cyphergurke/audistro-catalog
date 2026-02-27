package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"audistro-catalog/internal/httpapi/middleware"
)

func TestHealthzReturnsOKJSON(t *testing.T) {
	handler := middleware.RequestID(NewRouter(Dependencies{InsecureTransportAllowed: false}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", got)
	}

	var body HealthzResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.Status != "ok" {
		t.Fatalf("expected status ok, got %q", body.Status)
	}

	if body.RequestID == "" {
		t.Fatal("expected non-empty request_id")
	}
}

func TestHealthzEchoesRequestIDFromHeader(t *testing.T) {
	handler := middleware.RequestID(NewRouter(Dependencies{InsecureTransportAllowed: false}))

	const requestID = "req-test-123"
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", requestID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body HealthzResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.RequestID != requestID {
		t.Fatalf("expected request_id %q, got %q", requestID, body.RequestID)
	}

	if got := rec.Header().Get("X-Request-Id"); got != requestID {
		t.Fatalf("expected X-Request-Id header %q, got %q", requestID, got)
	}
}
