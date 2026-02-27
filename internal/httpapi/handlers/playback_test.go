package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPlaybackReturnsAssetAndProviders(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
	assetID := "asset-playback-1"

	assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      assetID,
		ArtistHandle: "alice",
		PayeeID:      payeeID,
		Title:        "Track Playback",
		DurationMS:   180000,
		ContentID:    "cid-playback-1",
		HLSMasterURL: "https://cdn.example/assets/asset-playback-1/master.m3u8",
		PriceMSat:    100,
	})
	if assetRec.Code != http.StatusCreated {
		t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
	}

	registerAndAnnounceProvider(t, app, "provider-pb-a", "08", assetID, 5, time.Now().Unix()+7200, "us")
	registerAndAnnounceProvider(t, app, "provider-pb-b", "09", assetID, 20, time.Now().Unix()+7200, "eu")

	rec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/playback/"+assetID+"?limit=1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp PlaybackResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode playback response: %v", err)
	}
	if resp.Now <= 0 {
		t.Fatalf("expected now to be set")
	}
	if resp.APIVersion != "v1" {
		t.Fatalf("expected api_version v1, got %q", resp.APIVersion)
	}
	if resp.SchemaVersion != 1 {
		t.Fatalf("expected schema_version 1, got %d", resp.SchemaVersion)
	}
	if resp.Asset.AssetID != assetID {
		t.Fatalf("expected asset_id %s, got %s", assetID, resp.Asset.AssetID)
	}
	if len(resp.Providers) != 1 {
		t.Fatalf("expected 1 provider due to limit, got %d", len(resp.Providers))
	}
	if resp.Providers[0].ProviderID != "provider-pb-b" {
		t.Fatalf("expected highest-ranked provider-pb-b, got %s", resp.Providers[0].ProviderID)
	}
	if resp.Asset.Pay == nil {
		t.Fatal("expected pay payload to be present")
	}
	if resp.Asset.Pay.PayeeID != payeeID {
		t.Fatalf("expected payee_id %q, got %q", payeeID, resp.Asset.Pay.PayeeID)
	}
	if resp.Asset.Pay.FAPPayeeID != "fap_alice" {
		t.Fatalf("expected fap_payee_id fap_alice, got %q", resp.Asset.Pay.FAPPayeeID)
	}
	if resp.Asset.Pay.FAPURL != "https://fap.artist.example" {
		t.Fatalf("expected fap_url https://fap.artist.example, got %q", resp.Asset.Pay.FAPURL)
	}
	if resp.Asset.Pay.PriceMSat != 100 {
		t.Fatalf("expected price_msat 100, got %d", resp.Asset.Pay.PriceMSat)
	}
}

func TestPlaybackETagNotModified(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
	assetID := "asset-playback-etag"

	assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      assetID,
		ArtistHandle: "alice",
		PayeeID:      payeeID,
		Title:        "Track ETag",
		DurationMS:   180000,
		ContentID:    "cid-playback-etag",
		HLSMasterURL: "https://cdn.example/assets/asset-playback-etag/master.m3u8",
		PriceMSat:    100,
	})
	if assetRec.Code != http.StatusCreated {
		t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
	}

	registerAndAnnounceProvider(t, app, "provider-etag", "0a", assetID, 10, time.Now().Unix()+7200, "us")

	firstReq := httptest.NewRequest(http.MethodGet, "/v1/playback/"+assetID, nil)
	firstReq.RemoteAddr = "127.0.0.1:1234"
	firstRec := doRequest(t, app.handler, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first playback failed: %d body=%s", firstRec.Code, firstRec.Body.String())
	}
	firstETag := firstRec.Header().Get("ETag")
	if firstETag == "" {
		t.Fatalf("expected ETag header")
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/v1/playback/"+assetID, nil)
	secondReq.Header.Set("If-None-Match", firstETag)
	secondReq.RemoteAddr = "127.0.0.1:1234"
	secondRec := doRequest(t, app.handler, secondReq)
	if secondRec.Code != http.StatusNotModified {
		t.Fatalf("expected 304, got %d body=%s", secondRec.Code, secondRec.Body.String())
	}

	if _, err := app.db.Exec(`UPDATE provider_assets SET updated_at = updated_at + 10 WHERE asset_id = ?`, assetID); err != nil {
		t.Fatalf("update provider_assets.updated_at: %v", err)
	}

	thirdReq := httptest.NewRequest(http.MethodGet, "/v1/playback/"+assetID, nil)
	thirdReq.Header.Set("If-None-Match", firstETag)
	thirdReq.RemoteAddr = "127.0.0.1:1234"
	thirdRec := doRequest(t, app.handler, thirdReq)
	if thirdRec.Code != http.StatusOK {
		t.Fatalf("expected 200 after update, got %d body=%s", thirdRec.Code, thirdRec.Body.String())
	}
	thirdETag := thirdRec.Header().Get("ETag")
	if thirdETag == firstETag {
		t.Fatalf("expected ETag to change after provider update")
	}
}

func TestPlaybackMissingAssetReturns404(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	rec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/playback/missing-asset", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPlaybackKeyURITemplateResolution(t *testing.T) {
	t.Parallel()

	t.Run("computed from payee base url", func(t *testing.T) {
		t.Parallel()
		app := newTestAppWithConfig(t, testAppConfig{playbackDefaultProviderLimit: 10, playbackMaxProviderLimit: 50})
		payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
		assetID := "asset-playback-key-payee"
		assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
			AssetID:       assetID,
			ArtistHandle:  "alice",
			PayeeID:       payeeID,
			Title:         "Track Key Payee",
			DurationMS:    180000,
			ContentID:     "cid-playback-key-payee",
			HLSMasterURL:  "https://cdn.example/assets/asset-playback-key-payee/master.m3u8",
			PreviewHLSURL: "https://asset.keys.example/{asset_id}/{key_id}",
			PriceMSat:     1,
		})
		if assetRec.Code != http.StatusCreated {
			t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
		}

		rec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/playback/"+assetID, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp PlaybackResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode playback response: %v", err)
		}
		if resp.Asset.HLS.KeyURITemplate != "https://fap.artist.example/hls/{asset_id}/key" {
			t.Fatalf("expected payee-derived key uri template, got %q", resp.Asset.HLS.KeyURITemplate)
		}
		if resp.Asset.Pay == nil {
			t.Fatal("expected pay payload to be present")
		}
		if resp.Asset.Pay.FAPURL != "https://fap.artist.example" {
			t.Fatalf("unexpected fap_url %q", resp.Asset.Pay.FAPURL)
		}
	})

	t.Run("returns 501 when payee missing", func(t *testing.T) {
		t.Parallel()
		app := newTestAppWithConfig(t, testAppConfig{playbackDefaultProviderLimit: 10, playbackMaxProviderLimit: 50})
		payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
		assetID := "asset-playback-key-missing-payee"
		assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
			AssetID:      assetID,
			ArtistHandle: "alice",
			PayeeID:      payeeID,
			Title:        "Track Missing Payee",
			DurationMS:   180000,
			ContentID:    "cid-playback-key-missing-payee",
			HLSMasterURL: "https://cdn.example/assets/asset-playback-key-missing-payee/master.m3u8",
			PriceMSat:    1,
		})
		if assetRec.Code != http.StatusCreated {
			t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
		}
		if _, err := app.db.Exec(`UPDATE payees SET fap_public_base_url = '' WHERE payee_id = ?`, payeeID); err != nil {
			t.Fatalf("invalidate payee base url: %v", err)
		}

		rec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/playback/"+assetID, nil)
		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("expected 501, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode playback response: %v", err)
		}
		if resp["error"] != "key_uri_template_unavailable" {
			t.Fatalf("unexpected error response: %v", resp)
		}
	})

	t.Run("prod-like rejects insecure payee base", func(t *testing.T) {
		t.Parallel()
		app := newTestAppWithConfig(t, testAppConfig{
			insecureTransportAllowed:     false,
			playbackDefaultProviderLimit: 10,
			playbackMaxProviderLimit:     50,
		})
		payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
		assetID := "asset-playback-key-http-prod"
		assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
			AssetID:      assetID,
			ArtistHandle: "alice",
			PayeeID:      payeeID,
			Title:        "Track Key HTTP",
			DurationMS:   180000,
			ContentID:    "cid-playback-key-http-prod",
			HLSMasterURL: "https://cdn.example/assets/asset-playback-key-http-prod/master.m3u8",
			PriceMSat:    1,
		})
		if assetRec.Code != http.StatusCreated {
			t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
		}
		if _, err := app.db.Exec(`UPDATE payees SET fap_public_base_url = 'http://fap.artist.example/' WHERE payee_id = ?`, payeeID); err != nil {
			t.Fatalf("update payee base url: %v", err)
		}

		rec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/playback/"+assetID, nil)
		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("expected 501, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode playback response: %v", err)
		}
		if resp["error"] != "key_uri_template_unavailable" {
			t.Fatalf("unexpected error response: %v", resp)
		}
	})

	t.Run("dev-like allows insecure payee base", func(t *testing.T) {
		t.Parallel()
		app := newTestAppWithConfig(t, testAppConfig{
			insecureTransportAllowed:     true,
			playbackDefaultProviderLimit: 10,
			playbackMaxProviderLimit:     50,
		})
		payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
		assetID := "asset-playback-key-http-dev"
		assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
			AssetID:      assetID,
			ArtistHandle: "alice",
			PayeeID:      payeeID,
			Title:        "Track Key HTTP Dev",
			DurationMS:   180000,
			ContentID:    "cid-playback-key-http-dev",
			HLSMasterURL: "https://cdn.example/assets/asset-playback-key-http-dev/master.m3u8",
			PriceMSat:    1,
		})
		if assetRec.Code != http.StatusCreated {
			t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
		}
		if _, err := app.db.Exec(`UPDATE payees SET fap_public_base_url = 'http://fap.artist.example/' WHERE payee_id = ?`, payeeID); err != nil {
			t.Fatalf("update payee base url: %v", err)
		}

		rec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/playback/"+assetID, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp PlaybackResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode playback response: %v", err)
		}
		if resp.Asset.HLS.KeyURITemplate != "http://fap.artist.example/hls/{asset_id}/key" {
			t.Fatalf("unexpected key uri template: %q", resp.Asset.HLS.KeyURITemplate)
		}
	})
}

func TestPlaybackProvidersTransportDerivedFromBaseURLScheme(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
	assetID := "asset-playback-transport-shape"

	assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      assetID,
		ArtistHandle: "alice",
		PayeeID:      payeeID,
		Title:        "Track Playback Transport",
		DurationMS:   180000,
		ContentID:    "cid-playback-transport-shape",
		HLSMasterURL: "https://cdn.example/assets/asset-playback-transport-shape/master.m3u8",
		PriceMSat:    100,
	})
	if assetRec.Code != http.StatusCreated {
		t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
	}

	registerAndAnnounceProvider(t, app, "provider-pb-transport", "0c", assetID, 10, time.Now().Unix()+7200, "us")
	if _, err := app.db.Exec(`UPDATE provider_assets SET transport = 'http' WHERE provider_id = ? AND asset_id = ?`, "provider-pb-transport", assetID); err != nil {
		t.Fatalf("update provider transport: %v", err)
	}

	rec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/playback/"+assetID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp PlaybackResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode playback response: %v", err)
	}
	if len(resp.Providers) != 1 {
		t.Fatalf("expected one provider, got %d", len(resp.Providers))
	}
	if resp.Providers[0].Transport != "https" {
		t.Fatalf("expected transport derived from base_url to be https, got %q", resp.Providers[0].Transport)
	}

	if _, err := app.db.Exec(`UPDATE provider_assets SET base_url = '://invalid' WHERE provider_id = ? AND asset_id = ?`, "provider-pb-transport", assetID); err != nil {
		t.Fatalf("update provider base_url: %v", err)
	}
	rec2 := doJSONRequest(t, app.handler, http.MethodGet, "/v1/playback/"+assetID, nil)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec2.Code, rec2.Body.String())
	}
	var resp2 PlaybackResponse
	if err := json.NewDecoder(rec2.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode playback response: %v", err)
	}
	if len(resp2.Providers) != 0 {
		t.Fatalf("expected invalid provider entry to be dropped, got %d providers", len(resp2.Providers))
	}
}
