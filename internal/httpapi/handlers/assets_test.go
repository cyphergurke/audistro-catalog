package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateAssetWithProviderHintsReturns201(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")

	rec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      "asset123",
		ArtistHandle: "alice",
		PayeeID:      payeeID,
		Title:        "Track A",
		DurationMS:   123000,
		ContentID:    "bafyasset123",
		HLSMasterURL: "https://cdn.example/assets/asset123/hls/master.m3u8",
		PriceMSat:    100000,
		ProviderHints: []CreateAssetProviderHintRequest{
			{Transport: "https", BaseURL: "https://cdn1.example/assets/asset123", Priority: 10},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp CreateAssetResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Asset.AssetID != "asset123" {
		t.Fatalf("expected asset_id asset123, got %q", resp.Asset.AssetID)
	}
	if len(resp.ProviderHints) != 1 {
		t.Fatalf("expected 1 provider hint, got %d", len(resp.ProviderHints))
	}
}

func TestCreateAssetWithPayeeMismatchReturns400(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	_ = createArtistAndPayeeForAssetTests(t, app, "alice")
	bobPayeeID := createArtistAndPayeeForAssetTests(t, app, "bob")

	rec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      "asset-mismatch",
		ArtistHandle: "alice",
		PayeeID:      bobPayeeID,
		Title:        "Track A",
		DurationMS:   123000,
		ContentID:    "cid-a",
		HLSMasterURL: "https://cdn.example/assets/a/master.m3u8",
		PriceMSat:    1,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetAssetReturnsPurchaseURLsFromPayee(t *testing.T) {
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
		t.Fatalf("create asset failed: status=%d body=%s", createRec.Code, createRec.Body.String())
	}

	getRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/assets/asset123", nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}

	var resp GetAssetResponse
	if err := json.NewDecoder(getRec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Asset.Purchase.ChallengeURL != "https://fap.artist.example/v1/fap/challenge" {
		t.Fatalf("unexpected challenge_url %q", resp.Asset.Purchase.ChallengeURL)
	}
	if resp.Asset.Purchase.TokenURL != "https://fap.artist.example/v1/fap/token" {
		t.Fatalf("unexpected token_url %q", resp.Asset.Purchase.TokenURL)
	}
	if resp.Asset.Purchase.ResourceID != "hls:key:asset123" {
		t.Fatalf("unexpected resource_id %q", resp.Asset.Purchase.ResourceID)
	}
}

func TestListArtistAssetsExcludesDelisted(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")

	_ = doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      "asset_allow",
		ArtistHandle: "alice",
		PayeeID:      payeeID,
		Title:        "Track A",
		DurationMS:   1000,
		ContentID:    "cid-a",
		HLSMasterURL: "https://cdn.example/a/master.m3u8",
		PriceMSat:    1,
	})
	_ = doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      "asset_delist",
		ArtistHandle: "alice",
		PayeeID:      payeeID,
		Title:        "Track B",
		DurationMS:   1000,
		ContentID:    "cid-b",
		HLSMasterURL: "https://cdn.example/b/master.m3u8",
		PriceMSat:    1,
	})

	if _, err := app.db.Exec(`INSERT INTO moderation_state(target_type, target_id, state, reason_code, updated_at) VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(target_type, target_id) DO UPDATE SET state=excluded.state, reason_code=excluded.reason_code, updated_at=excluded.updated_at`,
		"asset", "asset_delist", "delist", "test", 1); err != nil {
		t.Fatalf("insert moderation state: %v", err)
	}

	listRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/artists/alice/assets", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}

	var resp ListArtistAssetsResponse
	if err := json.NewDecoder(listRec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Assets) != 1 {
		t.Fatalf("expected 1 asset after delist filter, got %d", len(resp.Assets))
	}
	if resp.Assets[0].AssetID != "asset_allow" {
		t.Fatalf("expected asset_allow, got %q", resp.Assets[0].AssetID)
	}
}

func createArtistAndPayeeForAssetTests(t *testing.T, app *testApp, handle string) string {
	t.Helper()

	pubkey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if handle == "bob" {
		pubkey = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	}

	artistRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/artists", CreateArtistRequest{
		PubKeyHex:   pubkey,
		Handle:      handle,
		DisplayName: handle,
	})
	if artistRec.Code != http.StatusCreated {
		t.Fatalf("create artist failed status=%d body=%s", artistRec.Code, artistRec.Body.String())
	}

	payeeRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/payees", CreatePayeeRequest{
		ArtistHandle:     handle,
		FAPPublicBaseURL: "https://fap.artist.example",
		FAPPayeeID:       "fap_" + handle,
	})
	if payeeRec.Code != http.StatusCreated {
		t.Fatalf("create payee failed status=%d body=%s", payeeRec.Code, payeeRec.Body.String())
	}

	var payeeResp PayeeResponse
	if err := json.NewDecoder(payeeRec.Body).Decode(&payeeResp); err != nil {
		t.Fatalf("decode payee response: %v", err)
	}

	return payeeResp.Payee.PayeeID
}
