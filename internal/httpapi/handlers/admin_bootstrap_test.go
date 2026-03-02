package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestBootstrapArtistRequiresToken(t *testing.T) {
	t.Parallel()

	app := newTestAppWithConfig(t, testAppConfig{adminEnabled: true, adminToken: "secret"})
	rec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/admin/bootstrap/artist", BootstrapArtistRequest{
		Handle:      "artist_bootstrap",
		DisplayName: "Artist Bootstrap",
		Payee: BootstrapPayeeRequest{
			FAPPublicBaseURL: "http://localhost:18081",
			FAPPayeeID:       "fap_payee_bootstrap",
		},
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBootstrapArtistCreatesArtistAndPayee(t *testing.T) {
	t.Parallel()

	app := newTestAppWithConfig(t, testAppConfig{adminEnabled: true, adminToken: "secret"})
	payload, err := json.Marshal(BootstrapArtistRequest{
		Handle:      "artist_bootstrap",
		DisplayName: "Artist Bootstrap",
		Payee: BootstrapPayeeRequest{
			FAPPublicBaseURL: "http://localhost:18081",
			FAPPayeeID:       "fap_payee_bootstrap",
		},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := doBootstrapRequest(t, payload, "secret")
	rec := doRequest(t, app.handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp BootstrapArtistResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ArtistID == "" || resp.PayeeID == "" {
		t.Fatalf("expected ids in response: %+v", resp)
	}
	if resp.Handle != "artist_bootstrap" || resp.FAPPayeeID != "fap_payee_bootstrap" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	artistRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/artists/artist_bootstrap", nil)
	if artistRec.Code != http.StatusOK {
		t.Fatalf("expected artist lookup 200, got %d body=%s", artistRec.Code, artistRec.Body.String())
	}
	payeeRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/payees/"+resp.PayeeID, nil)
	if payeeRec.Code != http.StatusOK {
		t.Fatalf("expected payee lookup 200, got %d body=%s", payeeRec.Code, payeeRec.Body.String())
	}
}

func TestBootstrapArtistHandleConflict(t *testing.T) {
	t.Parallel()

	app := newTestAppWithConfig(t, testAppConfig{adminEnabled: true, adminToken: "secret"})
	firstPayload, err := json.Marshal(BootstrapArtistRequest{
		ArtistID:    "ar_one",
		Handle:      "artist_bootstrap",
		DisplayName: "Artist Bootstrap",
		Payee: BootstrapPayeeRequest{
			PayeeID:          "pe_one",
			FAPPublicBaseURL: "http://localhost:18081",
			FAPPayeeID:       "fap_payee_bootstrap",
		},
	})
	if err != nil {
		t.Fatalf("marshal first request: %v", err)
	}
	firstRec := doRequest(t, app.handler, doBootstrapRequest(t, firstPayload, "secret"))
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected first bootstrap 200, got %d body=%s", firstRec.Code, firstRec.Body.String())
	}

	secondPayload, err := json.Marshal(BootstrapArtistRequest{
		ArtistID:    "ar_two",
		Handle:      "artist_bootstrap",
		DisplayName: "Artist Bootstrap",
		Payee: BootstrapPayeeRequest{
			PayeeID:          "pe_two",
			FAPPublicBaseURL: "http://localhost:18081",
			FAPPayeeID:       "fap_payee_other",
		},
	})
	if err != nil {
		t.Fatalf("marshal second request: %v", err)
	}
	secondRec := doRequest(t, app.handler, doBootstrapRequest(t, secondPayload, "secret"))
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", secondRec.Code, secondRec.Body.String())
	}
}

func doBootstrapRequest(t *testing.T, payload []byte, token string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "/v1/admin/bootstrap/artist", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Admin-Token", token)
	}
	req.RemoteAddr = "127.0.0.1:1234"
	return req
}
