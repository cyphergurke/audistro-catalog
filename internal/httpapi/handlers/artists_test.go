package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"audistro-catalog/internal/httpapi/middleware"
	"audistro-catalog/internal/noncecache"
	"audistro-catalog/internal/providerhints"
	artistsvc "audistro-catalog/internal/service/artists"
	assetsvc "audistro-catalog/internal/service/assets"
	payeessvc "audistro-catalog/internal/service/payees"
	providersvc "audistro-catalog/internal/service/providers"
	reportsvc "audistro-catalog/internal/service/reports"
	"audistro-catalog/internal/store"
	storesqlite "audistro-catalog/internal/store/sqlite"
)

type testApp struct {
	handler http.Handler
	db      *sql.DB
}

type testAppConfig struct {
	defaultKeyURITemplate        string
	playbackDefaultProviderLimit int64
	playbackMaxProviderLimit     int64
	apiSchemaVersion             int
	httpMaxBodyBytes             int64
	etagMaxAgeSeconds            int64
	rateLimitAnnounceRPS         float64
	rateLimitAnnounceBurst       int64
	rateLimitPlaybackRPS         float64
	rateLimitPlaybackBurst       int64
	rateLimitCacheTTLSeconds     int64
	insecureTransportAllowed     bool
}

func newTestApp(t *testing.T) *testApp {
	return newTestAppWithConfig(t, testAppConfig{
		playbackDefaultProviderLimit: 10,
		playbackMaxProviderLimit:     50,
		apiSchemaVersion:             1,
		httpMaxBodyBytes:             32768,
		etagMaxAgeSeconds:            5,
		rateLimitAnnounceRPS:         5,
		rateLimitAnnounceBurst:       10,
		rateLimitPlaybackRPS:         20,
		rateLimitPlaybackBurst:       40,
		rateLimitCacheTTLSeconds:     600,
	})
}

func newTestAppWithConfig(t *testing.T, cfg testAppConfig) *testApp {
	t.Helper()
	if cfg.apiSchemaVersion <= 0 {
		cfg.apiSchemaVersion = 1
	}
	if cfg.httpMaxBodyBytes <= 0 {
		cfg.httpMaxBodyBytes = 32768
	}
	if cfg.etagMaxAgeSeconds <= 0 {
		cfg.etagMaxAgeSeconds = 5
	}
	if cfg.rateLimitAnnounceRPS <= 0 {
		cfg.rateLimitAnnounceRPS = 1000
	}
	if cfg.rateLimitAnnounceBurst <= 0 {
		cfg.rateLimitAnnounceBurst = 1000
	}
	if cfg.rateLimitPlaybackRPS <= 0 {
		cfg.rateLimitPlaybackRPS = 1000
	}
	if cfg.rateLimitPlaybackBurst <= 0 {
		cfg.rateLimitPlaybackBurst = 1000
	}
	if cfg.rateLimitCacheTTLSeconds <= 0 {
		cfg.rateLimitCacheTTLSeconds = 600
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "httpapi.sqlite")
	db, err := store.OpenSQLite(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	artistsRepo := storesqlite.NewArtistsRepo(db)
	moderationRepo := storesqlite.NewModerationRepo(db)
	payeesRepo := storesqlite.NewPayeesRepo(db)
	assetsRepo := storesqlite.NewAssetsRepo(db)
	providerHintsRepo := storesqlite.NewProviderHintsRepo(db)
	reportsRepo := storesqlite.NewReportsRepo(db)
	verificationRepo := storesqlite.NewVerificationRepo(db)
	providerRegistryRepo := storesqlite.NewProviderRegistryRepo(db)

	artistsService := artistsvc.NewService(artistsRepo, moderationRepo, verificationRepo)
	payeesService := payeessvc.NewService(payeesRepo, artistsRepo)
	assetsService := assetsvc.NewService(artistsRepo, payeesRepo, assetsRepo, providerHintsRepo, moderationRepo)
	reportsService := reportsvc.NewService(reportsRepo, moderationRepo, artistsRepo, verificationRepo)
	providersService := providersvc.NewService(providerRegistryRepo, assetsRepo, 1209600, 200, cfg.insecureTransportAllowed)
	providerHintsService := providerhints.NewService(providersService, providerhints.ServiceConfig{
		DefaultLimit: 20,
		MaxLimit:     100,
		Score: providerhints.Config{
			StaleThresholdSeconds: 86400,
			Recent10MBonus:        20,
			Recent1HBonus:         10,
			Old24HPenalty:         20,
			Expires1HPenalty:      30,
			Expires24HPenalty:     10,
			PriorityMultiplier:    3,
			PriorityMax:           30,
		},
	})
	nc := noncecache.New(100000)

	router := NewRouter(Dependencies{
		ArtistsService:               artistsService,
		PayeesService:                payeesService,
		AssetsService:                assetsService,
		ReportsService:               reportsService,
		ProvidersService:             providersService,
		ProviderHintsService:         providerHintsService,
		NonceCache:                   nc,
		NonceCacheTTLSeconds:         600,
		APIVersion:                   "v1",
		APISchemaVersion:             cfg.apiSchemaVersion,
		HTTPMaxBodyBytes:             cfg.httpMaxBodyBytes,
		ETagMaxAgeSeconds:            cfg.etagMaxAgeSeconds,
		RateLimitAnnounceRPS:         cfg.rateLimitAnnounceRPS,
		RateLimitAnnounceBurst:       cfg.rateLimitAnnounceBurst,
		RateLimitPlaybackRPS:         cfg.rateLimitPlaybackRPS,
		RateLimitPlaybackBurst:       cfg.rateLimitPlaybackBurst,
		RateLimitCacheTTLSeconds:     cfg.rateLimitCacheTTLSeconds,
		DefaultKeyURITemplate:        cfg.defaultKeyURITemplate,
		PlaybackDefaultProviderLimit: cfg.playbackDefaultProviderLimit,
		PlaybackMaxProviderLimit:     cfg.playbackMaxProviderLimit,
		InsecureTransportAllowed:     cfg.insecureTransportAllowed,
	})

	return &testApp{
		handler: middleware.RequestID(router),
		db:      db,
	}
}

func doJSONRequest(t *testing.T, handler http.Handler, method string, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		payload = encoded
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := doRequest(t, handler, req)
	return rec
}

func doRequest(t *testing.T, handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	if req.RemoteAddr == "" {
		req.RemoteAddr = "127.0.0.1:1234"
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestPostArtistThenGetByHandle(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	createRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/artists", CreateArtistRequest{
		PubKeyHex:   "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
		Handle:      "alice",
		DisplayName: "Alice",
		Bio:         "",
		AvatarURL:   "",
	})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	var createResp ArtistResponse
	if err := json.NewDecoder(createRec.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create artist response: %v", err)
	}

	getRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/artists/alice", nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}

	var getResp ArtistResponse
	if err := json.NewDecoder(getRec.Body).Decode(&getResp); err != nil {
		t.Fatalf("decode get artist response: %v", err)
	}

	if getResp.Artist.ArtistID != createResp.Artist.ArtistID {
		t.Fatalf("expected artist_id %q, got %q", createResp.Artist.ArtistID, getResp.Artist.ArtistID)
	}
	if getResp.Artist.Moderation.State != "allow" {
		t.Fatalf("expected moderation state allow, got %q", getResp.Artist.Moderation.State)
	}
	if getResp.Artist.Verification.Badge != "unverified" || getResp.Artist.Verification.Score != 0 {
		t.Fatalf("unexpected verification %+v", getResp.Artist.Verification)
	}
}

func TestPostArtistDuplicateHandleReturns409(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	first := doJSONRequest(t, app.handler, http.MethodPost, "/v1/artists", CreateArtistRequest{
		PubKeyHex:   "111122223333444455556666777788889999aaaabbbbccccddddeeeeffff0000",
		Handle:      "alice",
		DisplayName: "Alice",
	})
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first create 201, got %d body=%s", first.Code, first.Body.String())
	}

	second := doJSONRequest(t, app.handler, http.MethodPost, "/v1/artists", CreateArtistRequest{
		PubKeyHex:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Handle:      "alice",
		DisplayName: "Alice2",
	})
	if second.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", second.Code, second.Body.String())
	}
}
