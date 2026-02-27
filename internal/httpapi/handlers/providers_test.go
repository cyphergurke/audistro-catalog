package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

func TestProviderAnnounceSignatureSuccess(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
	assetID := "asset-provider-1"

	assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      assetID,
		ArtistHandle: "alice",
		PayeeID:      payeeID,
		Title:        "Track A",
		DurationMS:   123000,
		ContentID:    "cid-provider-1",
		HLSMasterURL: "https://cdn.example/assets/asset-provider-1/master.m3u8",
		PriceMSat:    10,
	})
	if assetRec.Code != http.StatusCreated {
		t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
	}

	privBytes, _ := hex.DecodeString("0101010101010101010101010101010101010101010101010101010101010101")
	privKey, pubKey := btcec.PrivKeyFromBytes(privBytes)
	providerID := "provider-1"
	publicKeyHex := hex.EncodeToString(pubKey.SerializeCompressed())

	registerRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers", RegisterProviderRequest{
		ProviderID: providerID,
		PublicKey:  publicKeyHex,
		Transport:  "https",
		BaseURL:    "https://provider.example",
		Region:     "eu",
	})
	if registerRec.Code != http.StatusOK {
		t.Fatalf("register provider failed: %d body=%s", registerRec.Code, registerRec.Body.String())
	}

	expiresAt := time.Now().Unix() + 3600
	nonce := "aabbccddeeff0011"
	announceBaseURL := "https://provider.example/assets/" + assetID
	signature := signAnnouncement(t, privKey, providerID, assetID, "https", announceBaseURL, expiresAt, nonce)

	announceRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers/"+providerID+"/announce", AnnounceProviderRequest{
		AssetID:   assetID,
		Transport: "https",
		BaseURL:   announceBaseURL,
		Priority:  10,
		ExpiresAt: expiresAt,
		Nonce:     nonce,
		Signature: signature,
	})
	if announceRec.Code != http.StatusOK {
		t.Fatalf("announce provider failed: %d body=%s", announceRec.Code, announceRec.Body.String())
	}

	listRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/assets/"+assetID+"/providers", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list providers failed: %d body=%s", listRec.Code, listRec.Body.String())
	}
	var listResp ListAssetProvidersResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list providers: %v", err)
	}
	if len(listResp.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(listResp.Providers))
	}
	if listResp.Providers[0].Transport != "https" {
		t.Fatalf("expected transport derived from base_url to be https, got %q", listResp.Providers[0].Transport)
	}
	if listResp.APIVersion != "v1" {
		t.Fatalf("expected api_version v1, got %q", listResp.APIVersion)
	}
	if listResp.SchemaVersion != 1 {
		t.Fatalf("expected schema_version 1, got %d", listResp.SchemaVersion)
	}
	if listResp.Providers[0].ProviderID != providerID {
		t.Fatalf("unexpected provider id %s", listResp.Providers[0].ProviderID)
	}
	if listResp.Now <= 0 {
		t.Fatalf("expected response now to be set")
	}
	if listResp.Providers[0].HintScore < 0 || listResp.Providers[0].HintScore > 100 {
		t.Fatalf("expected hint_score in range, got %d", listResp.Providers[0].HintScore)
	}
}

func TestProviderRegisterBodyLimit(t *testing.T) {
	t.Parallel()

	app := newTestAppWithConfig(t, testAppConfig{
		httpMaxBodyBytes:             64,
		playbackDefaultProviderLimit: 10,
		playbackMaxProviderLimit:     50,
	})

	oversizedBody := bytes.NewBufferString(`{"provider_id":"provider-body","public_key":"` + strings.Repeat("a", 66) + `","transport":"https","base_url":"https://` + strings.Repeat("x", 200) + `.example"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/providers", oversizedBody)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := doRequest(t, app.handler, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "body_too_large" {
		t.Fatalf("expected body_too_large, got %q", body["error"])
	}
}

func TestProviderAnnounceRateLimited(t *testing.T) {
	t.Parallel()

	app := newTestAppWithConfig(t, testAppConfig{
		rateLimitAnnounceRPS:         1,
		rateLimitAnnounceBurst:       1,
		playbackDefaultProviderLimit: 10,
		playbackMaxProviderLimit:     50,
	})
	payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
	assetID := "asset-provider-rl"

	assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      assetID,
		ArtistHandle: "alice",
		PayeeID:      payeeID,
		Title:        "Track RL",
		DurationMS:   123000,
		ContentID:    "cid-provider-rl",
		HLSMasterURL: "https://cdn.example/assets/asset-provider-rl/master.m3u8",
		PriceMSat:    10,
	})
	if assetRec.Code != http.StatusCreated {
		t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
	}

	privBytes, _ := hex.DecodeString("1212121212121212121212121212121212121212121212121212121212121212")
	privKey, pubKey := btcec.PrivKeyFromBytes(privBytes)
	providerID := "provider-rl"
	publicKeyHex := hex.EncodeToString(pubKey.SerializeCompressed())

	registerRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers", RegisterProviderRequest{
		ProviderID: providerID,
		PublicKey:  publicKeyHex,
		Transport:  "https",
		BaseURL:    "https://provider.rl.example",
	})
	if registerRec.Code != http.StatusOK {
		t.Fatalf("register provider failed: %d body=%s", registerRec.Code, registerRec.Body.String())
	}

	expiresAt := time.Now().Unix() + 3600
	baseURL := "https://provider.rl.example/assets/" + assetID
	firstNonce := "aaaabbbbccccdddd"
	firstSig := signAnnouncement(t, privKey, providerID, assetID, "https", baseURL, expiresAt, firstNonce)

	first := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers/"+providerID+"/announce", AnnounceProviderRequest{
		AssetID:   assetID,
		Transport: "https",
		BaseURL:   baseURL,
		Priority:  10,
		ExpiresAt: expiresAt,
		Nonce:     firstNonce,
		Signature: firstSig,
	})
	if first.Code != http.StatusOK {
		t.Fatalf("first announce failed: %d body=%s", first.Code, first.Body.String())
	}

	secondNonce := "eeeeffff11112222"
	secondSig := signAnnouncement(t, privKey, providerID, assetID, "https", baseURL, expiresAt, secondNonce)
	second := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers/"+providerID+"/announce", AnnounceProviderRequest{
		AssetID:   assetID,
		Transport: "https",
		BaseURL:   baseURL,
		Priority:  10,
		ExpiresAt: expiresAt,
		Nonce:     secondNonce,
		Signature: secondSig,
	})
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d body=%s", second.Code, second.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(second.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "rate_limited" {
		t.Fatalf("expected rate_limited, got %q", body["error"])
	}
}

func TestProviderAnnounceNonceReplayRejected(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
	assetID := "asset-provider-replay"

	assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      assetID,
		ArtistHandle: "alice",
		PayeeID:      payeeID,
		Title:        "Track Replay",
		DurationMS:   123000,
		ContentID:    "cid-provider-replay",
		HLSMasterURL: "https://cdn.example/assets/asset-provider-replay/master.m3u8",
		PriceMSat:    10,
	})
	if assetRec.Code != http.StatusCreated {
		t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
	}

	privBytes, _ := hex.DecodeString("1111111111111111111111111111111111111111111111111111111111111111")
	privKey, pubKey := btcec.PrivKeyFromBytes(privBytes)
	providerID := "provider-replay"
	publicKeyHex := hex.EncodeToString(pubKey.SerializeCompressed())

	registerRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers", RegisterProviderRequest{
		ProviderID: providerID,
		PublicKey:  publicKeyHex,
		Transport:  "https",
		BaseURL:    "https://provider.replay.example",
	})
	if registerRec.Code != http.StatusOK {
		t.Fatalf("register provider failed: %d body=%s", registerRec.Code, registerRec.Body.String())
	}

	expiresAt := time.Now().Unix() + 3600
	nonce := "deadbeef00112233"
	announceBaseURL := "https://provider.replay.example/assets/" + assetID
	signature := signAnnouncement(t, privKey, providerID, assetID, "https", announceBaseURL, expiresAt, nonce)
	payload := AnnounceProviderRequest{
		AssetID:   assetID,
		Transport: "https",
		BaseURL:   announceBaseURL,
		Priority:  10,
		ExpiresAt: expiresAt,
		Nonce:     nonce,
		Signature: signature,
	}

	first := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers/"+providerID+"/announce", payload)
	if first.Code != http.StatusOK {
		t.Fatalf("first announce failed: %d body=%s", first.Code, first.Body.String())
	}

	second := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers/"+providerID+"/announce", payload)
	if second.Code != http.StatusConflict {
		t.Fatalf("expected 409 on replay, got %d body=%s", second.Code, second.Body.String())
	}
	var replayResp struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(second.Body).Decode(&replayResp); err != nil {
		t.Fatalf("decode replay response: %v", err)
	}
	if replayResp.Error != "nonce_replay" {
		t.Fatalf("expected nonce_replay, got %q", replayResp.Error)
	}
}

func TestProviderAnnounceRejectedIfAssetMissing(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	privBytes, _ := hex.DecodeString("0202020202020202020202020202020202020202020202020202020202020202")
	privKey, pubKey := btcec.PrivKeyFromBytes(privBytes)
	providerID := "provider-404"
	publicKeyHex := hex.EncodeToString(pubKey.SerializeCompressed())

	registerRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers", RegisterProviderRequest{
		ProviderID: providerID,
		PublicKey:  publicKeyHex,
		Transport:  "https",
		BaseURL:    "https://provider404.example",
	})
	if registerRec.Code != http.StatusOK {
		t.Fatalf("register provider failed: %d body=%s", registerRec.Code, registerRec.Body.String())
	}

	assetID := "missing-asset"
	expiresAt := time.Now().Unix() + 3600
	nonce := "1122334455667788"
	announceBaseURL := "https://provider404.example/assets/" + assetID
	signature := signAnnouncement(t, privKey, providerID, assetID, "https", announceBaseURL, expiresAt, nonce)

	announceRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers/"+providerID+"/announce", AnnounceProviderRequest{
		AssetID:   assetID,
		Transport: "https",
		BaseURL:   announceBaseURL,
		Priority:  1,
		ExpiresAt: expiresAt,
		Nonce:     nonce,
		Signature: signature,
	})
	if announceRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", announceRec.Code, announceRec.Body.String())
	}
}

func TestAssetProvidersLimitAndRegionPreference(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
	assetID := "asset-provider-2"

	assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      assetID,
		ArtistHandle: "alice",
		PayeeID:      payeeID,
		Title:        "Track B",
		DurationMS:   123000,
		ContentID:    "cid-provider-2",
		HLSMasterURL: "https://cdn.example/assets/asset-provider-2/master.m3u8",
		PriceMSat:    10,
	})
	if assetRec.Code != http.StatusCreated {
		t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
	}

	registerAndAnnounceProvider(t, app, "provider-a", "03", assetID, 5, time.Now().Unix()+7200, "us")
	registerAndAnnounceProvider(t, app, "provider-b", "04", assetID, 20, time.Now().Unix()+7200, "eu")
	registerAndAnnounceProvider(t, app, "provider-c", "05", assetID, 10, time.Now().Unix()+7200, "us")

	listRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/assets/"+assetID+"/providers?region=us&limit=2", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list providers failed: %d body=%s", listRec.Code, listRec.Body.String())
	}
	var listResp ListAssetProvidersResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list providers: %v", err)
	}
	if len(listResp.Providers) != 2 {
		t.Fatalf("expected limit=2 providers, got %d", len(listResp.Providers))
	}
	if listResp.Providers[0].ProviderID != "provider-c" {
		t.Fatalf("expected provider-c first (region preferred), got %s", listResp.Providers[0].ProviderID)
	}
	if listResp.Providers[1].ProviderID != "provider-a" {
		t.Fatalf("expected provider-a second (region preferred), got %s", listResp.Providers[1].ProviderID)
	}
	if listResp.Providers[0].HintScore < listResp.Providers[1].HintScore {
		t.Fatalf("expected hint_score ordered desc within region preference")
	}
}

func TestAssetProvidersStaleFlagThreshold(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
	assetID := "asset-provider-stale"

	assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      assetID,
		ArtistHandle: "alice",
		PayeeID:      payeeID,
		Title:        "Track Stale",
		DurationMS:   123000,
		ContentID:    "cid-provider-stale",
		HLSMasterURL: "https://cdn.example/assets/asset-provider-stale/master.m3u8",
		PriceMSat:    10,
	})
	if assetRec.Code != http.StatusCreated {
		t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
	}

	registerAndAnnounceProvider(t, app, "provider-fresh", "06", assetID, 5, time.Now().Unix()+7200, "eu")
	registerAndAnnounceProvider(t, app, "provider-old", "07", assetID, 5, time.Now().Unix()+7200, "eu")

	oldLastSeen := time.Now().Unix() - (2 * 24 * 60 * 60)
	if _, err := app.db.Exec(`UPDATE provider_assets SET last_seen_at = ? WHERE provider_id = ? AND asset_id = ?`, oldLastSeen, "provider-old", assetID); err != nil {
		t.Fatalf("update old provider last_seen_at: %v", err)
	}

	listRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/assets/"+assetID+"/providers?limit=10", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list providers failed: %d body=%s", listRec.Code, listRec.Body.String())
	}
	var listResp ListAssetProvidersResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list providers: %v", err)
	}
	if len(listResp.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(listResp.Providers))
	}

	var freshFound bool
	var oldFound bool
	for _, provider := range listResp.Providers {
		if provider.ProviderID == "provider-fresh" {
			freshFound = true
			if provider.Stale {
				t.Fatalf("expected provider-fresh stale=false")
			}
		}
		if provider.ProviderID == "provider-old" {
			oldFound = true
			if !provider.Stale {
				t.Fatalf("expected provider-old stale=true")
			}
		}
	}
	if !freshFound || !oldFound {
		t.Fatalf("expected both providers in response")
	}
}

func TestAssetProvidersETagNotModified(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
	assetID := "asset-provider-etag"

	assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      assetID,
		ArtistHandle: "alice",
		PayeeID:      payeeID,
		Title:        "Track Provider ETag",
		DurationMS:   123000,
		ContentID:    "cid-provider-etag",
		HLSMasterURL: "https://cdn.example/assets/asset-provider-etag/master.m3u8",
		PriceMSat:    10,
	})
	if assetRec.Code != http.StatusCreated {
		t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
	}

	registerAndAnnounceProvider(t, app, "provider-etag-a", "0b", assetID, 10, time.Now().Unix()+7200, "us")

	firstReq := httptest.NewRequest(http.MethodGet, "/v1/assets/"+assetID+"/providers", nil)
	firstReq.RemoteAddr = "127.0.0.1:1234"
	firstRec := doRequest(t, app.handler, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first providers request failed: %d body=%s", firstRec.Code, firstRec.Body.String())
	}
	firstETag := firstRec.Header().Get("ETag")
	if firstETag == "" {
		t.Fatalf("expected ETag header")
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/v1/assets/"+assetID+"/providers", nil)
	secondReq.Header.Set("If-None-Match", firstETag)
	secondReq.RemoteAddr = "127.0.0.1:1234"
	secondRec := doRequest(t, app.handler, secondReq)
	if secondRec.Code != http.StatusNotModified {
		t.Fatalf("expected 304, got %d body=%s", secondRec.Code, secondRec.Body.String())
	}
}

func registerAndAnnounceProvider(t *testing.T, app *testApp, providerID string, privHexSuffix string, assetID string, priority int64, expiresAt int64, region string) {
	t.Helper()

	privHex := strings.Repeat(privHexSuffix, 64/len(privHexSuffix))
	privBytes, _ := hex.DecodeString(privHex)
	privKey, pubKey := btcec.PrivKeyFromBytes(privBytes)
	publicKeyHex := hex.EncodeToString(pubKey.SerializeCompressed())

	registerRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers", RegisterProviderRequest{
		ProviderID: providerID,
		PublicKey:  publicKeyHex,
		Transport:  "https",
		BaseURL:    "https://" + providerID + ".example",
		Region:     region,
	})
	if registerRec.Code != http.StatusOK {
		t.Fatalf("register provider %s failed: %d body=%s", providerID, registerRec.Code, registerRec.Body.String())
	}

	nonce := "abcdefabcdef1234" + privHexSuffix + privHexSuffix
	announceBaseURL := "https://" + providerID + ".example/assets/" + assetID
	signature := signAnnouncement(t, privKey, providerID, assetID, "https", announceBaseURL, expiresAt, nonce)

	announceRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers/"+providerID+"/announce", AnnounceProviderRequest{
		AssetID:   assetID,
		Transport: "https",
		BaseURL:   announceBaseURL,
		Priority:  priority,
		ExpiresAt: expiresAt,
		Nonce:     nonce,
		Signature: signature,
	})
	if announceRec.Code != http.StatusOK {
		t.Fatalf("announce provider %s failed: %d body=%s", providerID, announceRec.Code, announceRec.Body.String())
	}
}

func TestProviderRegisterHTTPPolicy(t *testing.T) {
	t.Parallel()

	// prod-like app: insecure transport not allowed
	app := newTestAppWithConfig(t, testAppConfig{insecureTransportAllowed: false})
	// attempt to register http transport should be rejected
	rec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers", RegisterProviderRequest{
		ProviderID: "prov-http-prod",
		PublicKey:  strings.Repeat("a", 66),
		Transport:  "http",
		BaseURL:    "http://prov.example",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 registering http in prod-like app, got %d body=%s", rec.Code, rec.Body.String())
	}

	// dev-like app: insecure allowed
	app2 := newTestAppWithConfig(t, testAppConfig{insecureTransportAllowed: true})
	pubkey := strings.Repeat("a", 66)
	rec2 := doJSONRequest(t, app2.handler, http.MethodPost, "/v1/providers", RegisterProviderRequest{
		ProviderID: "prov-http-dev",
		PublicKey:  pubkey,
		Transport:  "http",
		BaseURL:    "http://prov.example/",
	})
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 registering http in dev-like app, got %d body=%s", rec2.Code, rec2.Body.String())
	}
}

func TestProviderAnnounceHTTPPolicy(t *testing.T) {
	t.Parallel()

	runScenario := func(t *testing.T, insecureAllowed bool, expectStatus int) {
		t.Helper()
		app := newTestAppWithConfig(t, testAppConfig{insecureTransportAllowed: insecureAllowed})
		payeeID := createArtistAndPayeeForAssetTests(t, app, "alice")
		assetID := "asset-provider-http-policy"

		assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
			AssetID:      assetID,
			ArtistHandle: "alice",
			PayeeID:      payeeID,
			Title:        "Track HTTP Policy",
			DurationMS:   123000,
			ContentID:    "cid-provider-http-policy",
			HLSMasterURL: "https://cdn.example/assets/asset-provider-http-policy/master.m3u8",
			PriceMSat:    1,
		})
		if assetRec.Code != http.StatusCreated {
			t.Fatalf("create asset failed: %d body=%s", assetRec.Code, assetRec.Body.String())
		}

		privBytes, _ := hex.DecodeString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		privKey, pubKey := btcec.PrivKeyFromBytes(privBytes)
		providerID := "provider-http-policy"
		publicKeyHex := hex.EncodeToString(pubKey.SerializeCompressed())

		registerRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers", RegisterProviderRequest{
			ProviderID: providerID,
			PublicKey:  publicKeyHex,
			Transport:  "https",
			BaseURL:    "https://provider-http-policy.example",
		})
		if registerRec.Code != http.StatusOK {
			t.Fatalf("register provider failed: %d body=%s", registerRec.Code, registerRec.Body.String())
		}

		expiresAt := time.Now().Unix() + 3600
		nonce := "1234567890abcdef"
		announceBaseURL := "http://provider-http-policy.example/assets/" + assetID
		signature := signAnnouncement(t, privKey, providerID, assetID, "http", announceBaseURL, expiresAt, nonce)

		announceRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/providers/"+providerID+"/announce", AnnounceProviderRequest{
			AssetID:   assetID,
			Transport: "http",
			BaseURL:   announceBaseURL,
			Priority:  10,
			ExpiresAt: expiresAt,
			Nonce:     nonce,
			Signature: signature,
		})
		if announceRec.Code != expectStatus {
			t.Fatalf("expected announce status %d, got %d body=%s", expectStatus, announceRec.Code, announceRec.Body.String())
		}
	}

	t.Run("prod-like rejects http announce", func(t *testing.T) {
		runScenario(t, false, http.StatusBadRequest)
	})
	t.Run("dev-like allows http announce", func(t *testing.T) {
		runScenario(t, true, http.StatusOK)
	})
}

func signAnnouncement(t *testing.T, privKey *btcec.PrivateKey, providerID string, assetID string, transport string, baseURL string, expiresAt int64, nonce string) string {
	t.Helper()
	message := providerID + "|" + assetID + "|" + transport + "|" + baseURL + "|" + strconv.FormatInt(expiresAt, 10) + "|" + nonce
	hash := sha256.Sum256([]byte(message))
	sig, err := schnorr.Sign(privKey, hash[:])
	if err != nil {
		t.Fatalf("sign announcement: %v", err)
	}
	return hex.EncodeToString(sig.Serialize())
}
