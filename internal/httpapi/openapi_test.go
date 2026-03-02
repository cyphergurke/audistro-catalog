package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"audistro-catalog/internal/httpapi/handlers"
	middlewarepkg "audistro-catalog/internal/httpapi/middleware"
)

func testHandler() http.Handler {
	h := handlers.NewRouter(handlers.Dependencies{})
	logger := log.New(io.Discard, "", 0)
	handler := middlewarepkg.Recover(logger)(h)
	handler = middlewarepkg.AccessLog(logger)(handler)
	handler = openAPIValidationMiddleware(handler)
	handler = middlewarepkg.RequestID(handler)
	return handler
}

func TestOpenAPIEndpoints(t *testing.T) {
	h := testHandler()

	yamlReq := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	yamlRec := httptest.NewRecorder()
	h.ServeHTTP(yamlRec, yamlReq)
	if yamlRec.Code != http.StatusOK {
		t.Fatalf("expected /openapi.yaml to return 200, got %d", yamlRec.Code)
	}

	jsonReq := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	jsonRec := httptest.NewRecorder()
	h.ServeHTTP(jsonRec, jsonReq)
	if jsonRec.Code != http.StatusOK {
		t.Fatalf("expected /openapi.json to return 200, got %d", jsonRec.Code)
	}
	var doc map[string]any
	if err := json.Unmarshal(jsonRec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode json spec: %v", err)
	}
	if doc["openapi"] != "3.0.3" {
		t.Fatalf("unexpected openapi version: %#v", doc["openapi"])
	}

	docsReq := httptest.NewRequest(http.MethodGet, "/docs", nil)
	docsRec := httptest.NewRecorder()
	h.ServeHTTP(docsRec, docsReq)
	if docsRec.Code != http.StatusOK {
		t.Fatalf("expected /docs to return 200, got %d", docsRec.Code)
	}
	if body := docsRec.Body.String(); !bytes.Contains([]byte(body), []byte("data-url=\"/openapi.json\"")) {
		t.Fatalf("expected docs html to reference /openapi.json")
	}
}

func TestOpenAPIValidationRejectsWrongContentTypeForArtists(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/v1/artists", bytes.NewReader([]byte(`{"pubkey_hex":"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff","handle":"alice","display_name":"Alice"}`)))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var out validationAPIErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Error.Code != "invalid_request" {
		t.Fatalf("unexpected response: %#v", out)
	}
}

func TestOpenAPIValidationRejectsWrongContentTypeForProviderRegistration(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/v1/providers", bytes.NewReader([]byte(`{"provider_id":"prov1","public_key":"abc","transport":"http","base_url":"http://localhost:8080"}`)))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var out validationAPIErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Error.Code != "invalid_request" {
		t.Fatalf("unexpected response: %#v", out)
	}
}
