package ingest

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"audistro-catalog/internal/model"
	ingestsvc "audistro-catalog/internal/service/ingest"
	"audistro-catalog/internal/store"
	"audistro-catalog/internal/store/repo"
	storesqlite "audistro-catalog/internal/store/sqlite"
)

type fakeRunner struct {
	run func(ctx context.Context, name string, args ...string) error
}

func (f fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	return f.run(ctx, name, args...)
}

func TestProcessNextPublishesEncryptedAssetAndAnnounces(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, storagePath := openWorkerTestDB(t)
	artistsRepo := storesqlite.NewArtistsRepo(db)
	payeesRepo := storesqlite.NewPayeesRepo(db)
	assetsRepo := storesqlite.NewAssetsRepo(db)
	ingestJobsRepo := storesqlite.NewIngestJobsRepo(db)
	seedWorkerArtistAndPayee(t, ctx, artistsRepo, payeesRepo)
	service, err := ingestsvc.NewService(artistsRepo, payeesRepo, assetsRepo, ingestJobsRepo, storagePath, "http://localhost:18082")
	if err != nil {
		t.Fatalf("new ingest service: %v", err)
	}
	queued, err := service.QueueUpload(ctx, ingestsvc.QueueUploadInput{
		ArtistID:  "artist_worker",
		PayeeID:   "payee_worker",
		Title:     "Worker Track",
		PriceMSat: 1111,
		AssetID:   "asset_worker",
		Source:    bytes.NewReader([]byte("fake mp3")),
	})
	if err != nil {
		t.Fatalf("queue upload: %v", err)
	}

	providerRoot := t.TempDir()
	eu1Root := filepath.Join(providerRoot, "eu1")
	eu2Root := filepath.Join(providerRoot, "eu2")
	providerCalls := make([]string, 0, 4)
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerCalls = append(providerCalls, r.Host+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer providerServer.Close()

	packagingKey := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	fapCalls := make([]string, 0, 1)
	fapServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fapCalls = append(fapCalls, r.URL.Path)
		if r.Header.Get("X-Admin-Token") != "fap-admin-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(packagingKey)
	}))
	defer fapServer.Close()

	var ffmpegArgs []string
	worker, err := NewWorker(Dependencies{
		IngestService: service,
		Runner:        fakeRunner{run: fakeFFmpegRun(storagePath, &ffmpegArgs)},
		HTTPClient:    providerServer.Client(),
		StoragePath:   storagePath,
		ProviderTargets: []ProviderTarget{
			{Name: "eu_1", PublicBaseURL: "http://localhost:18082", InternalBaseURL: providerServer.URL + "/eu1", DataPathMount: eu1Root},
			{Name: "eu_2", PublicBaseURL: "http://localhost:18083", InternalBaseURL: providerServer.URL + "/eu2", DataPathMount: eu2Root},
		},
		FAPInternalBaseURL: fapServer.URL,
		FAPPublicBaseURL:   "http://localhost:18081",
		FAPAdminToken:      "fap-admin-token",
		WorkerPollInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}

	processed, err := worker.ProcessNext(ctx)
	if err != nil {
		t.Fatalf("process next: %v", err)
	}
	if !processed {
		t.Fatalf("expected worker to process a job")
	}

	job, err := ingestJobsRepo.GetIngestJob(ctx, model.IngestJobID(queued.JobID))
	if err != nil {
		t.Fatalf("get ingest job: %v", err)
	}
	if job.Status != model.IngestJobStatusPublished {
		t.Fatalf("expected published status, got %s", job.Status)
	}

	asset, err := assetsRepo.GetAsset(ctx, model.AssetID("asset_worker"))
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if asset.HLSMasterURL != "http://localhost:18082/assets/asset_worker/master.m3u8" {
		t.Fatalf("unexpected hls url %q", asset.HLSMasterURL)
	}
	if asset.DurationMS != 8000 {
		t.Fatalf("expected duration 8000ms, got %d", asset.DurationMS)
	}

	if len(fapCalls) != 1 || fapCalls[0] != "/internal/assets/asset_worker/packaging-key" {
		t.Fatalf("unexpected fap calls: %#v", fapCalls)
	}
	expectedProviderCalls := []string{
		strings.TrimPrefix(providerServer.URL, "http://") + "/eu1/internal/rescan",
		strings.TrimPrefix(providerServer.URL, "http://") + "/eu1/internal/announce",
		strings.TrimPrefix(providerServer.URL, "http://") + "/eu2/internal/rescan",
		strings.TrimPrefix(providerServer.URL, "http://") + "/eu2/internal/announce",
	}
	if !slices.Equal(providerCalls, expectedProviderCalls) {
		t.Fatalf("unexpected provider calls: %#v", providerCalls)
	}
	if !slices.Contains(ffmpegArgs, "-hls_key_info_file") {
		t.Fatalf("expected ffmpeg args to include -hls_key_info_file: %#v", ffmpegArgs)
	}

	for _, root := range []string{eu1Root, eu2Root} {
		masterPath := filepath.Join(root, "assets", "asset_worker", "master.m3u8")
		masterBytes, err := os.ReadFile(masterPath)
		if err != nil {
			t.Fatalf("read provider master playlist: %v", err)
		}
		masterText := string(masterBytes)
		if !strings.Contains(masterText, `#EXT-X-KEY:METHOD=AES-128,URI="http://localhost:18081/hls/asset_worker/key"`) {
			t.Fatalf("playlist missing AES-128 key line: %s", masterText)
		}
		if _, err := os.Stat(filepath.Join(root, "assets", "asset_worker", "seg_0000.ts")); err != nil {
			t.Fatalf("expected encrypted segment publish: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "assets", "asset_worker", "enc.key")); !os.IsNotExist(err) {
			t.Fatalf("enc.key leaked to provider dir: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "assets", "asset_worker", "enc.keyinfo")); !os.IsNotExist(err) {
			t.Fatalf("enc.keyinfo leaked to provider dir: %v", err)
		}
	}

	if !strings.Contains(job.PublishReport, `"published_targets":["eu_1","eu_2"]`) {
		t.Fatalf("unexpected publish report %q", job.PublishReport)
	}
}

func TestProcessNextPublishesWhenOneTargetFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, storagePath := openWorkerTestDB(t)
	artistsRepo := storesqlite.NewArtistsRepo(db)
	payeesRepo := storesqlite.NewPayeesRepo(db)
	assetsRepo := storesqlite.NewAssetsRepo(db)
	ingestJobsRepo := storesqlite.NewIngestJobsRepo(db)
	seedWorkerArtistAndPayee(t, ctx, artistsRepo, payeesRepo)
	service, err := ingestsvc.NewService(artistsRepo, payeesRepo, assetsRepo, ingestJobsRepo, storagePath, "http://localhost:18082")
	if err != nil {
		t.Fatalf("new ingest service: %v", err)
	}
	queued, err := service.QueueUpload(ctx, ingestsvc.QueueUploadInput{
		ArtistID:  "artist_worker",
		PayeeID:   "payee_worker",
		Title:     "Worker Track Partial",
		PriceMSat: 1111,
		AssetID:   "asset_worker_partial",
		Source:    bytes.NewReader([]byte("fake mp3")),
	})
	if err != nil {
		t.Fatalf("queue upload: %v", err)
	}

	providerRoot := t.TempDir()
	eu1Root := filepath.Join(providerRoot, "eu1")
	eu2Root := filepath.Join(providerRoot, "eu2")
	packagingKey := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	fapServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(packagingKey)
	}))
	defer fapServer.Close()

	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/eu2/") {
			http.Error(w, `{"error":"announce failed"}`, http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer providerServer.Close()

	var ffmpegArgs []string
	worker, err := NewWorker(Dependencies{
		IngestService: service,
		Runner:        fakeRunner{run: fakeFFmpegRunAsset("asset_worker_partial", storagePath, &ffmpegArgs)},
		HTTPClient:    providerServer.Client(),
		StoragePath:   storagePath,
		ProviderTargets: []ProviderTarget{
			{Name: "eu_1", PublicBaseURL: "http://localhost:18082", InternalBaseURL: providerServer.URL + "/eu1", DataPathMount: eu1Root},
			{Name: "eu_2", PublicBaseURL: "http://localhost:18083", InternalBaseURL: providerServer.URL + "/eu2", DataPathMount: eu2Root},
		},
		FAPInternalBaseURL: fapServer.URL,
		FAPPublicBaseURL:   "http://localhost:18081",
		FAPAdminToken:      "fap-admin-token",
	})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}

	processed, err := worker.ProcessNext(ctx)
	if err != nil {
		t.Fatalf("process next: %v", err)
	}
	if !processed {
		t.Fatalf("expected worker to process a job")
	}

	job, err := ingestJobsRepo.GetIngestJob(ctx, model.IngestJobID(queued.JobID))
	if err != nil {
		t.Fatalf("get ingest job: %v", err)
	}
	if job.Status != model.IngestJobStatusPublished {
		t.Fatalf("expected published status, got %s", job.Status)
	}
	if !strings.Contains(job.PublishReport, `"published_targets":["eu_1"]`) || !strings.Contains(job.PublishReport, `"name":"eu_2"`) {
		t.Fatalf("unexpected partial publish report %q", job.PublishReport)
	}
	if _, err := os.Stat(filepath.Join(eu1Root, "assets", "asset_worker_partial", "master.m3u8")); err != nil {
		t.Fatalf("expected successful publish on eu1: %v", err)
	}
	if _, err := os.Stat(filepath.Join(eu2Root, "assets", "asset_worker_partial", "master.m3u8")); err != nil {
		t.Fatalf("expected copied files on eu2 despite announce failure: %v", err)
	}
}

func openWorkerTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	storagePath := t.TempDir()
	dbPath := filepath.Join(storagePath, "catalog.sqlite")
	db, err := store.OpenSQLite(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db, storagePath
}

func seedWorkerArtistAndPayee(t *testing.T, ctx context.Context, artistsRepo repo.ArtistsRepository, payeesRepo repo.PayeesRepository) {
	t.Helper()
	_, err := artistsRepo.CreateArtist(ctx, repo.CreateArtistParams{
		ArtistID:    model.ArtistID("artist_worker"),
		PubKeyHex:   strings.Repeat("b", 64),
		Handle:      "worker-handle",
		DisplayName: "Worker Artist",
	})
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	_, err = payeesRepo.CreatePayee(ctx, repo.CreatePayeeParams{
		PayeeID:          model.PayeeID("payee_worker"),
		ArtistID:         model.ArtistID("artist_worker"),
		FAPPublicBaseURL: "http://localhost:18081",
		FAPPayeeID:       "fap_payee_worker",
	})
	if err != nil {
		t.Fatalf("create payee: %v", err)
	}
}

func fakeFFmpegRun(storagePath string, recordedArgs *[]string) func(ctx context.Context, name string, args ...string) error {
	return fakeFFmpegRunAsset("asset_worker", storagePath, recordedArgs)
}

func fakeFFmpegRunAsset(assetID string, storagePath string, recordedArgs *[]string) func(ctx context.Context, name string, args ...string) error {
	return func(ctx context.Context, name string, args ...string) error {
		if name != "ffmpeg" {
			return fmt.Errorf("unexpected command %s", name)
		}
		*recordedArgs = append([]string{}, args...)
		keyInfoPath := argValue(args, "-hls_key_info_file")
		if keyInfoPath == "" {
			return fmt.Errorf("missing -hls_key_info_file in args: %#v", args)
		}
		keyInfoBytes, err := os.ReadFile(keyInfoPath)
		if err != nil {
			return fmt.Errorf("read keyinfo: %w", err)
		}
		lines := strings.Split(strings.TrimSpace(string(keyInfoBytes)), "\n")
		if len(lines) != 3 {
			return fmt.Errorf("expected 3 keyinfo lines, got %d", len(lines))
		}
		keyBytes, err := os.ReadFile(lines[1])
		if err != nil {
			return fmt.Errorf("read enc key: %w", err)
		}
		if len(keyBytes) != 16 {
			return fmt.Errorf("expected 16-byte key, got %d", len(keyBytes))
		}
		if len(lines[2]) != 32 {
			return fmt.Errorf("expected 32-char iv, got %q", lines[2])
		}

		playlistPath := filepath.Join(storagePath, "build", assetID, "master.m3u8")
		segmentDir := filepath.Dir(playlistPath)
		if err := os.MkdirAll(segmentDir, 0o755); err != nil {
			return err
		}
		playlist := fmt.Sprintf("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-KEY:METHOD=AES-128,URI=\"%s\",IV=0x%s\n#EXTINF:4.000,\nseg_0000.ts\n#EXTINF:4.000,\nseg_0001.ts\n#EXT-X-ENDLIST\n", lines[0], lines[2])
		if err := os.WriteFile(playlistPath, []byte(playlist), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(segmentDir, "seg_0000.ts"), []byte("encseg0"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(segmentDir, "seg_0001.ts"), []byte("encseg1"), 0o644)
	}
}

func argValue(args []string, flag string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

func TestParsePlaylistDurationMSRoundsTinyPositiveDurationUpToOne(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "master.m3u8")
	content := "#EXTM3U\n#EXTINF:0.000011,\nseg_0000.ts\n#EXT-X-ENDLIST\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write playlist: %v", err)
	}

	durationMS, err := parsePlaylistDurationMS(path)
	if err != nil {
		t.Fatalf("parse playlist duration: %v", err)
	}
	if durationMS != 1 {
		t.Fatalf("expected 1ms for tiny positive duration, got %d", durationMS)
	}
}
