package jobs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"audistro-catalog/internal/model"
	"audistro-catalog/internal/store"
	"audistro-catalog/internal/store/repo"
	storesqlite "audistro-catalog/internal/store/sqlite"
)

func TestCleanupExpiredProviderAssetsOnce(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	db, err := store.OpenSQLite(ctx, filepath.Join(tmpDir, "cleanup.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	artistsRepo := storesqlite.NewArtistsRepo(db)
	payeesRepo := storesqlite.NewPayeesRepo(db)
	assetsRepo := storesqlite.NewAssetsRepo(db)
	providersRepo := storesqlite.NewProviderRegistryRepo(db)

	artist, err := artistsRepo.CreateArtist(ctx, repo.CreateArtistParams{
		ArtistID:    model.ArtistID("artist-cleanup"),
		PubKeyHex:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Handle:      "cleanupartist",
		DisplayName: "Cleanup Artist",
	})
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	payee, err := payeesRepo.CreatePayee(ctx, repo.CreatePayeeParams{
		PayeeID:          model.PayeeID("payee-cleanup"),
		ArtistID:         artist.ArtistID,
		FAPPublicBaseURL: "https://fap.cleanup.example",
		FAPPayeeID:       "fap-cleanup",
	})
	if err != nil {
		t.Fatalf("create payee: %v", err)
	}
	_, err = assetsRepo.CreateAsset(ctx, repo.CreateAssetParams{
		AssetID:      model.AssetID("asset-cleanup"),
		ArtistID:     artist.ArtistID,
		PayeeID:      payee.PayeeID,
		Title:        "cleanup",
		DurationMS:   1000,
		ContentID:    "cid-cleanup",
		HLSMasterURL: "https://cdn.cleanup.example/master.m3u8",
		PriceMSat:    1,
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	_, err = providersRepo.UpsertProvider(ctx, repo.UpsertProviderParams{
		ProviderID: "provider-cleanup",
		PublicKey:  "02aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Transport:  "https",
		BaseURL:    "https://provider.cleanup.example",
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("upsert provider: %v", err)
	}

	now := time.Now().Unix()
	_, err = providersRepo.UpsertProviderAssetAnnouncement(ctx, repo.UpsertProviderAssetAnnouncementParams{
		ProviderID: "provider-cleanup",
		AssetID:    model.AssetID("asset-cleanup"),
		Transport:  "https",
		BaseURL:    "https://provider.cleanup.example/assets/asset-cleanup",
		Priority:   1,
		ExpiresAt:  now - 5,
		LastSeenAt: now - 10,
		Nonce:      "aabbccddeeff0011",
	})
	if err != nil {
		t.Fatalf("upsert provider asset announcement: %v", err)
	}

	deleted, _, err := CleanupExpiredProviderAssetsOnce(ctx, providersRepo, time.Now())
	if err != nil {
		t.Fatalf("cleanup once: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted row, got %d", deleted)
	}
}
