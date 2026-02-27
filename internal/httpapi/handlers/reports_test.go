package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestArtistImpersonationDelistHidesFromBrowse(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	artistID, _, assetID, _ := createArtistPayeeAsset(t, app, "alice", "asset-alice", "01")

	for i := 0; i < 3; i++ {
		rec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/reports", CreateReportRequest{
			ReporterSubject: "reporter",
			TargetType:      "artist",
			TargetID:        artistID,
			ReportType:      "impersonation",
			Evidence:        "evidence",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create report status=%d body=%s", rec.Code, rec.Body.String())
		}
	}

	modRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/moderation/artist/"+artistID, nil)
	if modRec.Code != http.StatusOK {
		t.Fatalf("get moderation status=%d body=%s", modRec.Code, modRec.Body.String())
	}
	var modResp GetModerationResponse
	if err := json.NewDecoder(modRec.Body).Decode(&modResp); err != nil {
		t.Fatalf("decode moderation response: %v", err)
	}
	if modResp.Moderation.State != "delist" {
		t.Fatalf("expected delist moderation, got %q", modResp.Moderation.State)
	}

	browseArtists := doJSONRequest(t, app.handler, http.MethodGet, "/v1/browse/artists", nil)
	if browseArtists.Code != http.StatusOK {
		t.Fatalf("browse artists status=%d body=%s", browseArtists.Code, browseArtists.Body.String())
	}
	var artistsResp BrowseArtistsResponse
	if err := json.NewDecoder(browseArtists.Body).Decode(&artistsResp); err != nil {
		t.Fatalf("decode browse artists response: %v", err)
	}
	for _, artist := range artistsResp.Artists {
		if artist.ArtistID == artistID {
			t.Fatalf("delisted artist %s still visible in browse/artists", artistID)
		}
	}

	browseNew := doJSONRequest(t, app.handler, http.MethodGet, "/v1/browse/new", nil)
	if browseNew.Code != http.StatusOK {
		t.Fatalf("browse new status=%d body=%s", browseNew.Code, browseNew.Body.String())
	}
	var newResp BrowseNewResponse
	if err := json.NewDecoder(browseNew.Body).Decode(&newResp); err != nil {
		t.Fatalf("decode browse new response: %v", err)
	}
	for _, asset := range newResp.Assets {
		if asset.AssetID == assetID {
			t.Fatalf("asset %s from delisted artist still visible in browse/new", assetID)
		}
	}
}

func TestVerifiedArtistImpersonationBecomesWarn(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	artistID, pubKeyHex, _, _ := createArtistPayeeAsset(t, app, "verifiedalice", "asset-verified", "02")

	if _, err := app.db.Exec(
		`INSERT INTO verification_state(pubkey_hex, badge, score, updated_at) VALUES(?, ?, ?, ?)
		 ON CONFLICT(pubkey_hex) DO UPDATE SET badge=excluded.badge, score=excluded.score, updated_at=excluded.updated_at`,
		pubKeyHex, "verified", 90, time.Now().Unix(),
	); err != nil {
		t.Fatalf("insert verification_state: %v", err)
	}

	for i := 0; i < 3; i++ {
		rec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/reports", CreateReportRequest{
			ReporterSubject: "reporter",
			TargetType:      "artist",
			TargetID:        artistID,
			ReportType:      "impersonation",
			Evidence:        "evidence",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create report status=%d body=%s", rec.Code, rec.Body.String())
		}
	}

	modRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/moderation/artist/"+artistID, nil)
	if modRec.Code != http.StatusOK {
		t.Fatalf("get moderation status=%d body=%s", modRec.Code, modRec.Body.String())
	}
	var modResp GetModerationResponse
	if err := json.NewDecoder(modRec.Body).Decode(&modResp); err != nil {
		t.Fatalf("decode moderation response: %v", err)
	}
	if modResp.Moderation.State != "warn" {
		t.Fatalf("expected warn moderation, got %q", modResp.Moderation.State)
	}

	browseArtists := doJSONRequest(t, app.handler, http.MethodGet, "/v1/browse/artists", nil)
	if browseArtists.Code != http.StatusOK {
		t.Fatalf("browse artists status=%d body=%s", browseArtists.Code, browseArtists.Body.String())
	}
	var artistsResp BrowseArtistsResponse
	if err := json.NewDecoder(browseArtists.Body).Decode(&artistsResp); err != nil {
		t.Fatalf("decode browse artists response: %v", err)
	}

	found := false
	for _, artist := range artistsResp.Artists {
		if artist.ArtistID == artistID {
			found = true
			if artist.Moderation.State != "warn" {
				t.Fatalf("expected artist moderation warn, got %q", artist.Moderation.State)
			}
		}
	}
	if !found {
		t.Fatalf("verified artist %s should remain visible", artistID)
	}
}

func TestAssetScamThresholdQuarantineHidesFromBrowseNew(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	_, _, assetID, _ := createArtistPayeeAsset(t, app, "bob", "asset-scam", "03")

	for i := 0; i < 2; i++ {
		rec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/reports", CreateReportRequest{
			ReporterSubject: "reporter",
			TargetType:      "asset",
			TargetID:        assetID,
			ReportType:      "scam",
			Evidence:        "scam evidence",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create report status=%d body=%s", rec.Code, rec.Body.String())
		}
	}

	modRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/moderation/asset/"+assetID, nil)
	if modRec.Code != http.StatusOK {
		t.Fatalf("get moderation status=%d body=%s", modRec.Code, modRec.Body.String())
	}
	var modResp GetModerationResponse
	if err := json.NewDecoder(modRec.Body).Decode(&modResp); err != nil {
		t.Fatalf("decode moderation response: %v", err)
	}
	if modResp.Moderation.State != "quarantine" {
		t.Fatalf("expected quarantine moderation, got %q", modResp.Moderation.State)
	}

	browseNew := doJSONRequest(t, app.handler, http.MethodGet, "/v1/browse/new", nil)
	if browseNew.Code != http.StatusOK {
		t.Fatalf("browse new status=%d body=%s", browseNew.Code, browseNew.Body.String())
	}
	var newResp BrowseNewResponse
	if err := json.NewDecoder(browseNew.Body).Decode(&newResp); err != nil {
		t.Fatalf("decode browse new response: %v", err)
	}
	for _, asset := range newResp.Assets {
		if asset.AssetID == assetID {
			t.Fatalf("quarantined asset %s still visible in browse/new", assetID)
		}
	}
}

func TestURLMalwareQuarantine(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	targetURL := "https://malicious.example/path"

	rec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/reports", CreateReportRequest{
		ReporterSubject: "reporter",
		TargetType:      "url",
		TargetID:        targetURL,
		ReportType:      "malware",
		Evidence:        "malware",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create report status=%d body=%s", rec.Code, rec.Body.String())
	}

	stateRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/moderation/url/"+url.PathEscape(targetURL), nil)
	if stateRec.Code != http.StatusOK {
		t.Fatalf("get moderation status=%d body=%s", stateRec.Code, stateRec.Body.String())
	}
	var stateResp GetModerationResponse
	if err := json.NewDecoder(stateRec.Body).Decode(&stateResp); err != nil {
		t.Fatalf("decode moderation response: %v", err)
	}
	if stateResp.Moderation.State != "quarantine" {
		t.Fatalf("expected quarantine moderation, got %q", stateResp.Moderation.State)
	}
}

func createArtistPayeeAsset(t *testing.T, app *testApp, handle string, assetID string, suffix string) (string, string, string, string) {
	t.Helper()

	pubKeyHex := fmt.Sprintf("%s%s", strings.Repeat(suffix, 2), strings.Repeat("a", 62))
	if len(pubKeyHex) != 64 {
		pubKeyHex = strings.Repeat("b", 64)
	}

	artistRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/artists", CreateArtistRequest{
		PubKeyHex:   pubKeyHex,
		Handle:      handle,
		DisplayName: strings.ToUpper(handle),
	})
	if artistRec.Code != http.StatusCreated {
		t.Fatalf("create artist status=%d body=%s", artistRec.Code, artistRec.Body.String())
	}
	var artistResp ArtistResponse
	if err := json.NewDecoder(artistRec.Body).Decode(&artistResp); err != nil {
		t.Fatalf("decode artist response: %v", err)
	}

	payeeRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/payees", CreatePayeeRequest{
		ArtistHandle:     handle,
		FAPPublicBaseURL: "https://fap.artist.example",
		FAPPayeeID:       "fap_" + handle,
	})
	if payeeRec.Code != http.StatusCreated {
		t.Fatalf("create payee status=%d body=%s", payeeRec.Code, payeeRec.Body.String())
	}
	var payeeResp PayeeResponse
	if err := json.NewDecoder(payeeRec.Body).Decode(&payeeResp); err != nil {
		t.Fatalf("decode payee response: %v", err)
	}

	assetRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/assets", CreateAssetRequest{
		AssetID:      assetID,
		ArtistHandle: handle,
		PayeeID:      payeeResp.Payee.PayeeID,
		Title:        "Track",
		DurationMS:   120000,
		ContentID:    "cid-" + assetID,
		HLSMasterURL: "https://cdn.example/assets/" + assetID + "/master.m3u8",
		PriceMSat:    100,
	})
	if assetRec.Code != http.StatusCreated {
		t.Fatalf("create asset status=%d body=%s", assetRec.Code, assetRec.Body.String())
	}

	return artistResp.Artist.ArtistID, artistResp.Artist.PubKeyHex, assetID, payeeResp.Payee.PayeeID
}
