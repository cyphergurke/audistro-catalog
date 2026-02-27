package proofcheck

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/cyphergurke/audistro-catalog/internal/model"
	"github.com/cyphergurke/audistro-catalog/internal/store"
	"github.com/cyphergurke/audistro-catalog/internal/store/repo"
	storesqlite "github.com/cyphergurke/audistro-catalog/internal/store/sqlite"
)

type fakeDNSResolver struct {
	records map[string][]string
	err     error
}

func (f *fakeDNSResolver) LookupTXT(ctx context.Context, domain string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.records[domain], nil
}

type fakeHTTPResponse struct {
	status int
	body   []byte
	err    error
}

type fakeHTTPFetcher struct {
	responses map[string]fakeHTTPResponse
}

func (f *fakeHTTPFetcher) Get(ctx context.Context, rawURL string) (int, []byte, error) {
	resp, ok := f.responses[rawURL]
	if !ok {
		return 0, nil, errors.New("missing fake response")
	}
	if resp.err != nil {
		return 0, nil, resp.err
	}
	return resp.status, resp.body, nil
}

func TestDomainTXTSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	artistRepo := storesqlite.NewArtistsRepo(db)
	proofsRepo := storesqlite.NewProofsRepo(db)
	moderationRepo := storesqlite.NewModerationRepo(db)
	verificationRepo := storesqlite.NewVerificationRepo(db)

	pubKey := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	artistID := createArtist(t, ctx, artistRepo, pubKey, "alice")
	_ = artistID
	insertProof(t, db, "proof-1", pubKey, "domain_txt", "example.com", "pending")

	worker := mustWorker(t, Dependencies{
		ProofsRepo:       proofsRepo,
		ArtistsRepo:      artistRepo,
		ModerationRepo:   moderationRepo,
		VerificationRepo: verificationRepo,
		DNSResolver: &fakeDNSResolver{records: map[string][]string{
			"example.com": {"v=spf1", "fap-verification=" + pubKey},
		}},
		HTTPFetcher: &fakeHTTPFetcher{responses: map[string]fakeHTTPResponse{}},
		Limit:       10,
	})
	worker.nowFn = func() time.Time { return time.Unix(1700000000, 0) }

	if _, err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("run worker: %v", err)
	}

	assertProofStatus(t, db, "proof-1", "verified")
	state, err := verificationRepo.GetByPubKeyHex(ctx, pubKey)
	if err != nil {
		t.Fatalf("get verification: %v", err)
	}
	if state.Badge != "verified" || state.Score != 80 {
		t.Fatalf("unexpected verification state: badge=%s score=%d", state.Badge, state.Score)
	}
}

func TestWellKnownSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	artistRepo := storesqlite.NewArtistsRepo(db)
	proofsRepo := storesqlite.NewProofsRepo(db)
	moderationRepo := storesqlite.NewModerationRepo(db)
	verificationRepo := storesqlite.NewVerificationRepo(db)

	pubKey := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	createArtist(t, ctx, artistRepo, pubKey, "bob")
	insertProof(t, db, "proof-2", pubKey, "well_known", "artist.example", "pending")

	worker := mustWorker(t, Dependencies{
		ProofsRepo:       proofsRepo,
		ArtistsRepo:      artistRepo,
		ModerationRepo:   moderationRepo,
		VerificationRepo: verificationRepo,
		DNSResolver:      &fakeDNSResolver{records: map[string][]string{}},
		HTTPFetcher: &fakeHTTPFetcher{responses: map[string]fakeHTTPResponse{
			"https://artist.example/.well-known/fap.json": {
				status: 200,
				body:   []byte(fmt.Sprintf(`{"pubkey_hex":"%s","updated_at":123}`, pubKey)),
			},
		}},
		Limit: 10,
	})

	if _, err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("run worker: %v", err)
	}

	assertProofStatus(t, db, "proof-2", "verified")
}

func TestBothProofsVerifiedHighlyVerified(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	artistRepo := storesqlite.NewArtistsRepo(db)
	proofsRepo := storesqlite.NewProofsRepo(db)
	moderationRepo := storesqlite.NewModerationRepo(db)
	verificationRepo := storesqlite.NewVerificationRepo(db)

	pubKey := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	createArtist(t, ctx, artistRepo, pubKey, "carol")
	insertProof(t, db, "proof-3", pubKey, "domain_txt", "example.com", "pending")
	insertProof(t, db, "proof-4", pubKey, "well_known", "example.com", "pending")

	worker := mustWorker(t, Dependencies{
		ProofsRepo:       proofsRepo,
		ArtistsRepo:      artistRepo,
		ModerationRepo:   moderationRepo,
		VerificationRepo: verificationRepo,
		DNSResolver: &fakeDNSResolver{records: map[string][]string{
			"example.com": {"fap-verification=" + pubKey},
		}},
		HTTPFetcher: &fakeHTTPFetcher{responses: map[string]fakeHTTPResponse{
			"https://example.com/.well-known/fap.json": {
				status: 200,
				body:   []byte(fmt.Sprintf(`{"pubkey_hex":"%s","updated_at":456}`, pubKey)),
			},
		}},
		Limit: 10,
	})

	if _, err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("run worker: %v", err)
	}

	state, err := verificationRepo.GetByPubKeyHex(ctx, pubKey)
	if err != nil {
		t.Fatalf("get verification: %v", err)
	}
	if state.Badge != "highly_verified" || state.Score != 120 {
		t.Fatalf("unexpected verification state: badge=%s score=%d", state.Badge, state.Score)
	}
}

func TestImpersonationModerationOverridesVerification(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	artistRepo := storesqlite.NewArtistsRepo(db)
	proofsRepo := storesqlite.NewProofsRepo(db)
	moderationRepo := storesqlite.NewModerationRepo(db)
	verificationRepo := storesqlite.NewVerificationRepo(db)

	pubKey := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	artistID := createArtist(t, ctx, artistRepo, pubKey, "dana")
	insertProof(t, db, "proof-5", pubKey, "domain_txt", "example.com", "pending")

	_, err := moderationRepo.UpsertState(ctx, repo.UpsertModerationStateParams{
		TargetType: "artist",
		TargetID:   artistID,
		State:      "delist",
		ReasonCode: "impersonation_threshold",
	})
	if err != nil {
		t.Fatalf("upsert moderation state: %v", err)
	}

	worker := mustWorker(t, Dependencies{
		ProofsRepo:       proofsRepo,
		ArtistsRepo:      artistRepo,
		ModerationRepo:   moderationRepo,
		VerificationRepo: verificationRepo,
		DNSResolver: &fakeDNSResolver{records: map[string][]string{
			"example.com": {"fap-verification=" + pubKey},
		}},
		HTTPFetcher: &fakeHTTPFetcher{responses: map[string]fakeHTTPResponse{}},
		Limit:       10,
	})

	if _, err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("run worker: %v", err)
	}

	state, err := verificationRepo.GetByPubKeyHex(ctx, pubKey)
	if err != nil {
		t.Fatalf("get verification: %v", err)
	}
	if state.Badge != "flagged" || state.Score != 0 {
		t.Fatalf("unexpected verification state: badge=%s score=%d", state.Badge, state.Score)
	}
}

func TestWorkerIdempotency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	artistRepo := storesqlite.NewArtistsRepo(db)
	proofsRepo := storesqlite.NewProofsRepo(db)
	moderationRepo := storesqlite.NewModerationRepo(db)
	verificationRepo := storesqlite.NewVerificationRepo(db)

	pubKey := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	createArtist(t, ctx, artistRepo, pubKey, "erin")
	insertProof(t, db, "proof-6", pubKey, "domain_txt", "example.com", "pending")

	worker := mustWorker(t, Dependencies{
		ProofsRepo:       proofsRepo,
		ArtistsRepo:      artistRepo,
		ModerationRepo:   moderationRepo,
		VerificationRepo: verificationRepo,
		DNSResolver: &fakeDNSResolver{records: map[string][]string{
			"example.com": {"fap-verification=" + pubKey},
		}},
		HTTPFetcher: &fakeHTTPFetcher{responses: map[string]fakeHTTPResponse{}},
		Limit:       10,
	})

	if _, err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("first run worker: %v", err)
	}

	beforeProof := readProofRow(t, db, "proof-6")
	beforeState, err := verificationRepo.GetByPubKeyHex(ctx, pubKey)
	if err != nil {
		t.Fatalf("get verification before second run: %v", err)
	}

	if _, err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("second run worker: %v", err)
	}

	afterProof := readProofRow(t, db, "proof-6")
	afterState, err := verificationRepo.GetByPubKeyHex(ctx, pubKey)
	if err != nil {
		t.Fatalf("get verification after second run: %v", err)
	}

	if beforeProof != afterProof {
		t.Fatalf("proof row changed across idempotent runs: before=%v after=%v", beforeProof, afterProof)
	}
	if beforeState != afterState {
		t.Fatalf("verification state changed across idempotent runs: before=%+v after=%+v", beforeState, afterState)
	}
}

func mustWorker(t *testing.T, deps Dependencies) *Worker {
	t.Helper()
	worker, err := NewWorker(deps)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	return worker
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "worker.sqlite")
	db, err := store.OpenSQLite(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func createArtist(t *testing.T, ctx context.Context, artistsRepo repo.ArtistsRepository, pubKey string, handle string) string {
	t.Helper()
	artist, err := artistsRepo.CreateArtist(ctx, repo.CreateArtistParams{
		ArtistID:    model.ArtistID("art_" + handle),
		PubKeyHex:   pubKey,
		Handle:      handle,
		DisplayName: handle,
	})
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	return string(artist.ArtistID)
}

func insertProof(t *testing.T, db *sql.DB, proofID string, pubKey string, proofType string, proofValue string, status string) {
	t.Helper()
	now := time.Now().Unix()
	if _, err := db.Exec(
		`INSERT INTO artist_proofs(proof_id, artist_pubkey_hex, proof_type, proof_value, status, checked_at, details, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		proofID, pubKey, proofType, proofValue, status, 0, "", now, now,
	); err != nil {
		t.Fatalf("insert proof: %v", err)
	}
}

func assertProofStatus(t *testing.T, db *sql.DB, proofID string, expected string) {
	t.Helper()
	var status string
	if err := db.QueryRow(`SELECT status FROM artist_proofs WHERE proof_id = ?`, proofID).Scan(&status); err != nil {
		t.Fatalf("query proof status: %v", err)
	}
	if status != expected {
		t.Fatalf("expected proof status %q, got %q", expected, status)
	}
}

type proofRow struct {
	status    string
	checkedAt int64
	details   string
	updatedAt int64
}

func readProofRow(t *testing.T, db *sql.DB, proofID string) proofRow {
	t.Helper()
	row := proofRow{}
	if err := db.QueryRow(`SELECT status, checked_at, details, updated_at FROM artist_proofs WHERE proof_id = ?`, proofID).Scan(
		&row.status,
		&row.checkedAt,
		&row.details,
		&row.updatedAt,
	); err != nil {
		t.Fatalf("read proof row: %v", err)
	}
	return row
}
