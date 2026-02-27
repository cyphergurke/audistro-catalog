package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"audistro-catalog/internal/model"
	"audistro-catalog/internal/store"
	"audistro-catalog/internal/store/repo"
	"audistro-catalog/internal/store/sqlite"
)

func TestCreateArtistDuplicateHandleFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	artists := sqlite.NewArtistsRepo(db)

	_, err := artists.CreateArtist(ctx, repo.CreateArtistParams{
		ArtistID:    model.ArtistID("artist-1"),
		PubKeyHex:   "pubkey-1",
		Handle:      "alice",
		DisplayName: "Alice",
	})
	if err != nil {
		t.Fatalf("create first artist: %v", err)
	}

	_, err = artists.CreateArtist(ctx, repo.CreateArtistParams{
		ArtistID:    model.ArtistID("artist-2"),
		PubKeyHex:   "pubkey-2",
		Handle:      "alice",
		DisplayName: "Alice 2",
	})
	if err == nil {
		t.Fatal("expected duplicate handle error, got nil")
	}
}

func TestCreatePayeeReferencingArtist(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	artists := sqlite.NewArtistsRepo(db)
	payees := sqlite.NewPayeesRepo(db)

	createdArtist, err := artists.CreateArtist(ctx, repo.CreateArtistParams{
		ArtistID:    model.ArtistID("artist-1"),
		PubKeyHex:   "pubkey-1",
		Handle:      "alice",
		DisplayName: "Alice",
	})
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}

	createdPayee, err := payees.CreatePayee(ctx, repo.CreatePayeeParams{
		PayeeID:          model.PayeeID("payee-1"),
		ArtistID:         createdArtist.ArtistID,
		FAPPublicBaseURL: "https://artist.example",
		FAPPayeeID:       "fap-payee-1",
	})
	if err != nil {
		t.Fatalf("create payee: %v", err)
	}

	fetched, err := payees.GetPayee(ctx, createdPayee.PayeeID)
	if err != nil {
		t.Fatalf("get payee: %v", err)
	}
	if fetched.ArtistID != createdArtist.ArtistID {
		t.Fatalf("expected artist_id %q, got %q", createdArtist.ArtistID, fetched.ArtistID)
	}
}

func TestCreateAssetReferencingPayee(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	artists := sqlite.NewArtistsRepo(db)
	payees := sqlite.NewPayeesRepo(db)
	assets := sqlite.NewAssetsRepo(db)

	createdArtist, err := artists.CreateArtist(ctx, repo.CreateArtistParams{
		ArtistID:    model.ArtistID("artist-1"),
		PubKeyHex:   "pubkey-1",
		Handle:      "alice",
		DisplayName: "Alice",
	})
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}

	createdPayee, err := payees.CreatePayee(ctx, repo.CreatePayeeParams{
		PayeeID:          model.PayeeID("payee-1"),
		ArtistID:         createdArtist.ArtistID,
		FAPPublicBaseURL: "https://artist.example",
		FAPPayeeID:       "fap-payee-1",
	})
	if err != nil {
		t.Fatalf("create payee: %v", err)
	}

	createdAsset, err := assets.CreateAsset(ctx, repo.CreateAssetParams{
		AssetID:      model.AssetID("asset-1"),
		ArtistID:     createdArtist.ArtistID,
		PayeeID:      createdPayee.PayeeID,
		Title:        "Track",
		DurationMS:   120000,
		ContentID:    "cid-1",
		HLSMasterURL: "https://cdn.example/master.m3u8",
		PriceMSat:    250,
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}

	fetched, err := assets.GetAsset(ctx, createdAsset.AssetID)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if fetched.PayeeID != createdPayee.PayeeID {
		t.Fatalf("expected payee_id %q, got %q", createdPayee.PayeeID, fetched.PayeeID)
	}
}

func TestProviderHintsOrderedByPriority(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	artists := sqlite.NewArtistsRepo(db)
	payees := sqlite.NewPayeesRepo(db)
	assets := sqlite.NewAssetsRepo(db)
	hints := sqlite.NewProviderHintsRepo(db)

	createdArtist, err := artists.CreateArtist(ctx, repo.CreateArtistParams{
		ArtistID:    model.ArtistID("artist-1"),
		PubKeyHex:   "pubkey-1",
		Handle:      "alice",
		DisplayName: "Alice",
	})
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}

	createdPayee, err := payees.CreatePayee(ctx, repo.CreatePayeeParams{
		PayeeID:          model.PayeeID("payee-1"),
		ArtistID:         createdArtist.ArtistID,
		FAPPublicBaseURL: "https://artist.example",
		FAPPayeeID:       "fap-payee-1",
	})
	if err != nil {
		t.Fatalf("create payee: %v", err)
	}

	createdAsset, err := assets.CreateAsset(ctx, repo.CreateAssetParams{
		AssetID:      model.AssetID("asset-1"),
		ArtistID:     createdArtist.ArtistID,
		PayeeID:      createdPayee.PayeeID,
		Title:        "Track",
		DurationMS:   120000,
		ContentID:    "cid-1",
		HLSMasterURL: "https://cdn.example/master.m3u8",
		PriceMSat:    250,
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}

	_, err = hints.AddHint(ctx, repo.AddProviderHintParams{
		HintID:    model.ProviderHintID("hint-2"),
		AssetID:   createdAsset.AssetID,
		Transport: "https",
		BaseURL:   "https://b.example",
		Priority:  2,
	})
	if err != nil {
		t.Fatalf("add hint 2: %v", err)
	}

	_, err = hints.AddHint(ctx, repo.AddProviderHintParams{
		HintID:    model.ProviderHintID("hint-1"),
		AssetID:   createdAsset.AssetID,
		Transport: "https",
		BaseURL:   "https://a.example",
		Priority:  1,
	})
	if err != nil {
		t.Fatalf("add hint 1: %v", err)
	}

	listed, err := hints.ListHintsByAsset(ctx, createdAsset.AssetID)
	if err != nil {
		t.Fatalf("list hints: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 hints, got %d", len(listed))
	}
	if listed[0].HintID != model.ProviderHintID("hint-1") {
		t.Fatalf("expected first hint hint-1, got %q", listed[0].HintID)
	}
	if listed[1].HintID != model.ProviderHintID("hint-2") {
		t.Fatalf("expected second hint hint-2, got %q", listed[1].HintID)
	}
}

func TestModerationUpsertOverwritesPreviousValue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	moderation := sqlite.NewModerationRepo(db)

	_, err := moderation.UpsertState(ctx, repo.UpsertModerationStateParams{
		TargetType: "asset",
		TargetID:   "asset-1",
		State:      "allow",
		ReasonCode: "initial",
	})
	if err != nil {
		t.Fatalf("upsert state 1: %v", err)
	}

	_, err = moderation.UpsertState(ctx, repo.UpsertModerationStateParams{
		TargetType: "asset",
		TargetID:   "asset-1",
		State:      "block",
		ReasonCode: "dmca",
	})
	if err != nil {
		t.Fatalf("upsert state 2: %v", err)
	}

	state, ok, err := moderation.GetState(ctx, "asset", "asset-1")
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if !ok {
		t.Fatal("expected moderation state to exist")
	}
	if state.State != "block" {
		t.Fatalf("expected state block, got %q", state.State)
	}
	if state.ReasonCode != "dmca" {
		t.Fatalf("expected reason_code dmca, got %q", state.ReasonCode)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	db, err := store.OpenSQLite(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}
