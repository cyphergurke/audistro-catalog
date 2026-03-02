package handlers

import (
	"net/http"
	"time"

	"audistro-catalog/internal/apidocs"
	"audistro-catalog/internal/noncecache"
	"audistro-catalog/internal/providerhints"
	"audistro-catalog/internal/ratelimit"
	artistsvc "audistro-catalog/internal/service/artists"
	assetsvc "audistro-catalog/internal/service/assets"
	bootstrapsvc "audistro-catalog/internal/service/bootstrap"
	ingestsvc "audistro-catalog/internal/service/ingest"
	payeessvc "audistro-catalog/internal/service/payees"
	providersvc "audistro-catalog/internal/service/providers"
	reportsvc "audistro-catalog/internal/service/reports"
)

type Dependencies struct {
	ArtistsService               *artistsvc.Service
	PayeesService                *payeessvc.Service
	AssetsService                *assetsvc.Service
	BootstrapService             *bootstrapsvc.Service
	IngestService                *ingestsvc.Service
	ReportsService               *reportsvc.Service
	ProvidersService             *providersvc.Service
	ProviderHintsService         *providerhints.Service
	NonceCache                   *noncecache.Cache
	NonceCacheTTLSeconds         int64
	APIVersion                   string
	APISchemaVersion             int
	ETagMaxAgeSeconds            int64
	HTTPMaxBodyBytes             int64
	RateLimitAnnounceRPS         float64
	RateLimitAnnounceBurst       int64
	RateLimitPlaybackRPS         float64
	RateLimitPlaybackBurst       int64
	RateLimitCacheTTLSeconds     int64
	DefaultKeyURITemplate        string
	PlaybackDefaultProviderLimit int64
	PlaybackMaxProviderLimit     int64
	InsecureTransportAllowed     bool
	AdminEnabled                 bool
	AdminToken                   string
	AdminUploadMaxBodyBytes      int64
}

// NewRouter registers API routes.
func NewRouter(deps Dependencies) *http.ServeMux {
	if deps.APIVersion == "" {
		deps.APIVersion = "v1"
	}
	if deps.APISchemaVersion <= 0 {
		deps.APISchemaVersion = 1
	}
	if deps.ETagMaxAgeSeconds <= 0 {
		deps.ETagMaxAgeSeconds = 5
	}

	mux := http.NewServeMux()
	announceLimiter := ratelimit.New(deps.RateLimitAnnounceRPS, int(deps.RateLimitAnnounceBurst), time.Duration(deps.RateLimitCacheTTLSeconds)*time.Second)
	playbackLimiter := ratelimit.New(deps.RateLimitPlaybackRPS, int(deps.RateLimitPlaybackBurst), time.Duration(deps.RateLimitCacheTTLSeconds)*time.Second)

	mux.Handle("GET /openapi.yaml", apidocs.YAMLHandler())
	mux.Handle("GET /openapi.json", apidocs.JSONHandler())
	mux.Handle("GET /docs", apidocs.DocsHandler())
	mux.Handle("GET /docs/", apidocs.DocsHandler())
	mux.HandleFunc("GET /healthz", Healthz)
	mux.HandleFunc("POST /v1/admin/bootstrap/artist", BootstrapArtistHandler(deps))
	mux.HandleFunc("POST /v1/admin/assets/upload", AdminUploadAssetHandler(deps))
	mux.HandleFunc("GET /v1/admin/ingest/jobs/{jobId}", GetIngestJobHandler(deps))
	mux.HandleFunc("POST /v1/artists", CreateArtistHandler(deps.ArtistsService))
	mux.HandleFunc("GET /v1/artists/{handle}", GetArtistByHandleHandler(deps.ArtistsService))
	mux.HandleFunc("POST /v1/payees", CreatePayeeHandler(deps.PayeesService))
	mux.HandleFunc("GET /v1/payees/{payeeId}", GetPayeeHandler(deps.PayeesService))
	mux.HandleFunc("GET /v1/artists/{handle}/payees", ListArtistPayeesHandler(deps.PayeesService))
	mux.HandleFunc("POST /v1/assets", CreateAssetHandler(deps.AssetsService))
	mux.HandleFunc("GET /v1/assets/{assetId}", GetAssetHandler(deps.AssetsService))
	mux.HandleFunc("GET /v1/artists/{handle}/assets", ListArtistAssetsHandler(deps.AssetsService))
	mux.HandleFunc("POST /v1/assets/{assetId}/provider-hints", AddProviderHintHandler(deps.AssetsService))
	mux.HandleFunc("GET /v1/assets/{assetId}/provider-hints", ListProviderHintsHandler(deps.AssetsService))
	mux.HandleFunc("POST /v1/reports", CreateReportHandler(deps.ReportsService))
	mux.HandleFunc("GET /v1/moderation/{targetType}/{targetId...}", GetModerationHandler(deps.ReportsService))
	mux.HandleFunc("GET /v1/browse/artists", BrowseArtistsHandler(deps.ArtistsService))
	mux.HandleFunc("GET /v1/browse/new", BrowseNewHandler(deps.ArtistsService, deps.AssetsService))
	registerProviderHandler := withBodyLimit(RegisterProviderHandler(deps.ProvidersService, deps.InsecureTransportAllowed), deps.HTTPMaxBodyBytes)
	announceProviderHandler := withBodyLimit(AnnounceProviderHandler(deps.ProvidersService, deps.NonceCache, deps.NonceCacheTTLSeconds, deps.InsecureTransportAllowed), deps.HTTPMaxBodyBytes)
	announceProviderHandler = withRateLimit(announceProviderHandler, announceLimiter)
	playbackHandler := GetPlaybackHandler(
		deps.AssetsService,
		deps.ProviderHintsService,
		deps.PayeesService,
		deps.DefaultKeyURITemplate,
		deps.PlaybackDefaultProviderLimit,
		deps.PlaybackMaxProviderLimit,
		deps.APIVersion,
		deps.APISchemaVersion,
		deps.ETagMaxAgeSeconds,
		deps.InsecureTransportAllowed,
	)
	playbackHandler = withRateLimit(playbackHandler, playbackLimiter)

	mux.HandleFunc("POST /v1/providers", registerProviderHandler)
	mux.HandleFunc("POST /v1/providers/{providerId}/announce", announceProviderHandler)
	mux.HandleFunc("GET /v1/assets/{assetId}/providers", ListAssetProvidersHandler(deps.AssetsService, deps.ProviderHintsService, deps.APIVersion, deps.APISchemaVersion, deps.ETagMaxAgeSeconds))
	mux.HandleFunc(
		"GET /v1/playback/{assetId}",
		playbackHandler,
	)
	return mux
}
