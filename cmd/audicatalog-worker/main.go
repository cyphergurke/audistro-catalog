package main

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/cyphergurke/audistro-catalog/internal/config"
	"github.com/cyphergurke/audistro-catalog/internal/store"
	storesqlite "github.com/cyphergurke/audistro-catalog/internal/store/sqlite"
	"github.com/cyphergurke/audistro-catalog/internal/worker/proofcheck"
)

func main() {
	cfg := config.LoadFromEnv()
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	db, err := store.OpenSQLite(context.Background(), cfg.DBPath)
	if err != nil {
		logger.Fatalf("open database failed: %v", err)
	}
	defer db.Close()

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
