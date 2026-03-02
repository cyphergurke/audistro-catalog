package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"audistro-catalog/internal/apidocs"
	"audistro-catalog/internal/httpapi/handlers"
	middlewarepkg "audistro-catalog/internal/httpapi/middleware"
)

func testHandler() http.Handler {
	return testHandlerWithReadOnly(false)
}

func testHandlerWithReadOnly(readOnly bool) http.Handler {
	h := handlers.NewRouter(handlers.Dependencies{})
	logger := log.New(io.Discard, "", 0)
	handler := middlewarepkg.Recover(logger)(h)
	handler = middlewarepkg.AccessLog(logger)(handler)
	handler = middlewarepkg.OpenAPIValidate(middlewarepkg.OpenAPIValidateConfig{
		LoadSpec:        apidocs.LoadSpec,
		IncludePrefixes: []string{"/v1/"},
	})(handler)
	handler = middlewarepkg.ReadOnly(middlewarepkg.ReadOnlyConfig{Enabled: readOnly})(handler)
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

func TestAdminRoutesReturn404WhenDisabled(t *testing.T) {
	h := handlers.NewRouter(handlers.Dependencies{
		AdminEnabled: false,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/bootstrap/artist", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when admin routes disabled, got %d body=%s", rec.Code, rec.Body.String())
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
	assertInvalidRequest(t, rec)
}

func TestOpenAPIValidationRejectsMissingRequiredFieldForBootstrapArtist(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/bootstrap/artist", bytes.NewReader([]byte(`{"handle":"alice","payee":{"fap_public_base_url":"http://localhost:18081","fap_payee_id":"fap_alice"}}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Token", "dev-admin-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertInvalidRequest(t, rec)
}

func TestOpenAPIValidationRejectsMissingMultipartAudioPart(t *testing.T) {
	h := testHandler()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("artist_id", "artist_1")
	_ = writer.WriteField("payee_id", "payee_1")
	_ = writer.WriteField("title", "Smoke Upload")
	_ = writer.WriteField("price_msat", "1000")
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/assets/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Admin-Token", "dev-admin-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertInvalidRequest(t, rec)
}

func TestOpenAPIValidationRejectsInvalidAssetIDPathBeforeHandler(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/playback/bad!id", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertInvalidRequest(t, rec)
}

func TestReadOnlyRejectsMutatingProviderRequest(t *testing.T) {
	h := testHandlerWithReadOnly(true)
	req := httptest.NewRequest(http.MethodPost, "/v1/providers", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "read_only" {
		t.Fatalf("unexpected response: %#v", payload)
	}
}

func TestReadOnlyAllowsPlaybackRead(t *testing.T) {
	h := testHandlerWithReadOnly(true)
	req := httptest.NewRequest(http.MethodGet, "/v1/playback/asset_ok", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("expected read endpoint to bypass read-only block")
	}
}

func assertInvalidRequest(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "invalid_request" {
		t.Fatalf("unexpected response: %#v", payload)
	}
	if message, _ := payload["message"].(string); message == "" {
		t.Fatalf("expected validation message, got %#v", payload)
	}
}
