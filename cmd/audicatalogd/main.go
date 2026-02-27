package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"audistro-catalog/internal/config"
	"audistro-catalog/internal/httpapi"
	"audistro-catalog/internal/httpapi/handlers"
	"audistro-catalog/internal/jobs"
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

func main() {
	cfg := config.LoadFromEnv()
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	logger.Printf("starting audicatalogd env=%s addr=%s", cfg.Env, cfg.HTTPAddr)

	db, err := store.OpenSQLite(context.Background(), cfg.DBPath)
	if err != nil {
		logger.Fatalf("open database failed: %v", err)
	}
	defer db.Close()

	artistsRepo := storesqlite.NewArtistsRepo(db)
	moderationRepo := storesqlite.NewModerationRepo(db)
	payeesRepo := storesqlite.NewPayeesRepo(db)
	assetsRepo := storesqlite.NewAssetsRepo(db)
	providerHintsRepo := storesqlite.NewProviderHintsRepo(db)
	reportsRepo := storesqlite.NewReportsRepo(db)
	verificationRepo := storesqlite.NewVerificationRepo(db)
	providerRegistryRepo := storesqlite.NewProviderRegistryRepo(db)

	nonceCache := noncecache.New(int(cfg.NonceCacheMaxEntries))
	jobsCtx, jobsCancel := context.WithCancel(context.Background())
	defer jobsCancel()
	nonceCache.StartJanitor(jobsCtx, time.Duration(cfg.NonceCacheTTLSeconds)*time.Second)
	jobs.StartCleanupExpiredProviderAssets(jobsCtx, logger, providerRegistryRepo, time.Duration(cfg.CleanupIntervalSeconds)*time.Second)

	artistsService := artistsvc.NewService(artistsRepo, moderationRepo, verificationRepo)
	payeesService := payeessvc.NewService(payeesRepo, artistsRepo)
	assetsService := assetsvc.NewService(artistsRepo, payeesRepo, assetsRepo, providerHintsRepo, moderationRepo)
	reportsService := reportsvc.NewService(reportsRepo, moderationRepo, artistsRepo, verificationRepo)
	providersService := providersvc.NewService(providerRegistryRepo, assetsRepo, cfg.MaxAnnounceTTLSeconds, cfg.MaxProvidersPerAsset, cfg.IsInsecureTransportAllowed())
	providerHintsService := providerhints.NewService(providersService, providerhints.ServiceConfig{
		DefaultLimit: cfg.ProvidersQueryDefaultLimit,
		MaxLimit:     cfg.ProvidersQueryMaxLimit,
		Score: providerhints.Config{
			StaleThresholdSeconds: cfg.ProviderStaleThresholdSeconds,
			Recent10MBonus:        int(cfg.ProviderScoreRecent10MBonus),
			Recent1HBonus:         int(cfg.ProviderScoreRecent1HBonus),
			Old24HPenalty:         int(cfg.ProviderScoreOld24HPenalty),
			Expires1HPenalty:      int(cfg.ProviderScoreExpires1HPenalty),
			Expires24HPenalty:     int(cfg.ProviderScoreExpires24HPenalty),
			PriorityMultiplier:    int(cfg.ProviderScorePriorityMultiplier),
			PriorityMax:           int(cfg.ProviderScorePriorityMax),
		},
	})

	srv := httpapi.NewServer(cfg, logger, handlers.Dependencies{
		ArtistsService:               artistsService,
		PayeesService:                payeesService,
		AssetsService:                assetsService,
		ReportsService:               reportsService,
		ProvidersService:             providersService,
		ProviderHintsService:         providerHintsService,
		NonceCache:                   nonceCache,
		NonceCacheTTLSeconds:         cfg.NonceCacheTTLSeconds,
		APIVersion:                   "v1",
		APISchemaVersion:             cfg.APISchemaVersion,
		ETagMaxAgeSeconds:            cfg.ETagMaxAgeSeconds,
		HTTPMaxBodyBytes:             cfg.HTTPMaxBodyBytes,
		RateLimitAnnounceRPS:         cfg.RateLimitAnnounceRPS,
		RateLimitAnnounceBurst:       cfg.RateLimitAnnounceBurst,
		RateLimitPlaybackRPS:         cfg.RateLimitPlaybackRPS,
		RateLimitPlaybackBurst:       cfg.RateLimitPlaybackBurst,
		RateLimitCacheTTLSeconds:     cfg.RateLimitCacheTTLSeconds,
		DefaultKeyURITemplate:        cfg.DefaultKeyURITemplate,
		PlaybackDefaultProviderLimit: cfg.PlaybackDefaultProviderLimit,
		PlaybackMaxProviderLimit:     cfg.PlaybackMaxProviderLimit,
		InsecureTransportAllowed:     cfg.IsInsecureTransportAllowed(),
	})
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatalf("server failed: %v", err)
	}
}
