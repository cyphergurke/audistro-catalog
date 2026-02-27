package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestAddAndListProviderHintsOrdered(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
	createRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      "asset123",
		ArtistHandle: "alice",
		PayeeID:      payeeID,
		Title:        "Track A",
		DurationMS:   123000,
		ContentID:    "cid-123",
		HLSMasterURL: "https://cdn.example/assets/asset123/hls/master.m3u8",
		PriceMSat:    100000,
	})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create asset failed: %d body=%s", createRec.Code, createRec.Body.String())
	}

	addRec1 := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets/asset123/provider-hints", AddProviderHintRequest{
		Transport: "https",
		BaseURL:   "https://cdn-low.example/assets/asset123",
		Priority:  10,
	})
	if addRec1.Code != http.StatusCreated {
		t.Fatalf("add hint1 failed: %d body=%s", addRec1.Code, addRec1.Body.String())
	}

	addRec2 := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets/asset123/provider-hints", AddProviderHintRequest{
		Transport: "https",
		BaseURL:   "https://cdn-high.example/assets/asset123",
		Priority:  90,
	})
	if addRec2.Code != http.StatusCreated {
		t.Fatalf("add hint2 failed: %d body=%s", addRec2.Code, addRec2.Body.String())
	}

	listRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/assets/asset123/provider-hints", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list hints failed: %d body=%s", listRec.Code, listRec.Body.String())
	}

	var listResp ListProviderHintsResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Hints) < 2 {
		t.Fatalf("expected at least 2 hints, got %d", len(listResp.Hints))
	}
	if listResp.Hints[0].Priority < listResp.Hints[1].Priority {
		t.Fatalf("expected descending priority, got %d then %d", listResp.Hints[0].Priority, listResp.Hints[1].Priority)
	}
}
