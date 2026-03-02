package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"audistro-catalog/internal/config"
	ingestsvc "audistro-catalog/internal/service/ingest"
	"audistro-catalog/internal/store"
	storesqlite "audistro-catalog/internal/store/sqlite"
	ingestworker "audistro-catalog/internal/worker/ingest"
	"audistro-catalog/internal/worker/proofcheck"
)

func main() {
	cfg := config.LoadFromEnv()
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	db, err := store.OpenSQLite(context.Background(), cfg.DBPath)
	if err != nil {
		logger.Fatalf("open database failed: %v", err)
	}
	defer db.Close()

	mode := strings.TrimSpace(os.Getenv("AUDICATALOG_WORKER_MODE"))
	if mode == "" {
		mode = "proofcheck"
	}

	switch mode {
	case "ingest":
		runIngestWorker(logger, cfg, db)
	case "proofcheck":
		runProofcheckWorker(logger, db)
	default:
		logger.Fatalf("unsupported AUDICATALOG_WORKER_MODE: %q", mode)
	}
}

func runProofcheckWorker(logger *log.Logger, db *sql.DB) {
	limit := 200
	if rawLimit := os.Getenv("AUDICATALOG_WORKER_LIMIT"); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsed <= 0 {
			logger.Fatalf("invalid AUDICATALOG_WORKER_LIMIT: %q", rawLimit)
		}
		limit = parsed
	}

	worker, err := proofcheck.NewWorker(proofcheck.Dependencies{
		ProofsRepo:       storesqlite.NewProofsRepo(db),
		ArtistsRepo:      storesqlite.NewArtistsRepo(db),
		ModerationRepo:   storesqlite.NewModerationRepo(db),
		VerificationRepo: storesqlite.NewVerificationRepo(db),
		DNSResolver:      proofcheck.NewNetDNSResolver(),
		HTTPFetcher:      proofcheck.NewNetHTTPFetcher(),
		Limit:            limit,
	})
	if err != nil {
		logger.Fatalf("build worker failed: %v", err)
	}

	result, err := worker.RunOnce(context.Background())
	if err != nil {
		logger.Fatalf("worker run failed: %v", err)
	}

	logger.Printf("verification pass complete processed_proofs=%d affected_artists=%d", result.ProcessedProofs, result.AffectedArtists)
}

func runIngestWorker(logger *log.Logger, cfg config.Config, db *sql.DB) {
	artistsRepo := storesqlite.NewArtistsRepo(db)
	payeesRepo := storesqlite.NewPayeesRepo(db)
	assetsRepo := storesqlite.NewAssetsRepo(db)
	ingestJobsRepo := storesqlite.NewIngestJobsRepo(db)

	primaryTarget, err := cfg.PrimaryProviderTarget()
	if err != nil {
		logger.Fatalf("resolve primary provider target failed: %v", err)
	}

	ingestService, err := ingestsvc.NewService(artistsRepo, payeesRepo, assetsRepo, ingestJobsRepo, cfg.StoragePath, primaryTarget.PublicBaseURL)
	if err != nil {
		logger.Fatalf("build ingest service failed: %v", err)
	}

	providerTargets := make([]ingestworker.ProviderTarget, 0, len(cfg.ProviderTargets))
	for _, target := range cfg.ProviderTargets {
		providerTargets = append(providerTargets, ingestworker.ProviderTarget{
			Name:            target.Name,
			PublicBaseURL:   target.PublicBaseURL,
			InternalBaseURL: target.InternalBaseURL,
			DataPathMount:   target.DataPathMount,
		})
	}

	worker, err := ingestworker.NewWorker(ingestworker.Dependencies{
		IngestService:            ingestService,
		Runner:                   ingestworker.ExecRunner{},
		HTTPClient:               &http.Client{Timeout: 15 * time.Second},
		Logger:                   logger,
		StoragePath:              cfg.StoragePath,
		ProviderTargets:          providerTargets,
		FAPInternalBaseURL:       cfg.FAPInternalBaseURL,
		FAPPublicBaseURL:         cfg.FAPPublicBaseURL,
		FAPAdminToken:            cfg.FAPAdminToken,
		WorkerPollInterval:       time.Duration(cfg.WorkerPollIntervalSeconds) * time.Second,
		StaleProcessingThreshold: time.Duration(cfg.WorkerStaleSeconds) * time.Second,
	})
	if err != nil {
		logger.Fatalf("build ingest worker failed: %v", err)
	}

	logger.Printf("starting audicatalog ingest worker poll_interval=%ds provider_targets=%d primary_public=%s", cfg.WorkerPollIntervalSeconds, len(cfg.ProviderTargets), primaryTarget.PublicBaseURL)
	if err := worker.Run(context.Background()); err != nil && err != context.Canceled {
		logger.Fatalf("ingest worker failed: %v", err)
	}
}
