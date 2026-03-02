package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"audistro-catalog/internal/model"
	"audistro-catalog/internal/store/repo"
	storesqlite "audistro-catalog/internal/store/sqlite"
)

func TestAdminUploadAssetRequiresToken(t *testing.T) {
	t.Parallel()

	app := newTestAppWithConfig(t, testAppConfig{adminEnabled: true, adminToken: "secret", adminUploadMaxBodyBytes: 1 << 20})
	seedArtistAndPayee(t, app.db)

	req := newUploadRequest(t, "/v1/admin/assets/upload", "artist_upload", "payee_upload", "Track", "1000", "", []byte("fake mp3"))
	rec := doRequest(t, app.handler, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminUploadAssetQueuesJobAndWritesFile(t *testing.T) {
	t.Parallel()

	app := newTestAppWithConfig(t, testAppConfig{adminEnabled: true, adminToken: "secret", adminUploadMaxBodyBytes: 1 << 20})
	seedArtistAndPayee(t, app.db)

	req := newUploadRequest(t, "/v1/admin/assets/upload", "artist_upload", "payee_upload", "Track", "1000", "asset_upload", []byte("fake mp3 bytes"))
	req.Header.Set("X-Admin-Token", "secret")
	rec := doRequest(t, app.handler, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp adminUploadResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if resp.AssetID != "asset_upload" {
		t.Fatalf("expected asset_upload, got %q", resp.AssetID)
	}
	if resp.Status != "queued" || resp.JobID == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	job := fetchIngestJob(t, app.db, resp.JobID)
	if job.AssetID != model.AssetID("asset_upload") || job.Status != model.IngestJobStatusQueued {
		t.Fatalf("unexpected job row: %+v", job)
	}
	payload, err := os.ReadFile(job.SourcePath)
	if err != nil {
		t.Fatalf("read stored upload: %v", err)
	}
	if string(payload) != "fake mp3 bytes" {
		t.Fatalf("unexpected stored payload %q", string(payload))
	}
}

func TestGetIngestJobReturnsStatusShape(t *testing.T) {
	t.Parallel()

	app := newTestAppWithConfig(t, testAppConfig{adminEnabled: true, adminToken: "secret", adminUploadMaxBodyBytes: 1 << 20})
	seedArtistAndPayee(t, app.db)
	jobRepo := storesqlite.NewIngestJobsRepo(app.db)
	created, err := jobRepo.CreateIngestJob(context.Background(), repo.CreateIngestJobParams{
		JobID:      model.IngestJobID("job_lookup"),
		AssetID:    model.AssetID("asset_lookup"),
		ArtistID:   model.ArtistID("artist_upload"),
		PayeeID:    model.PayeeID("payee_upload"),
		Title:      "Lookup",
		PriceMSat:  123,
		SourcePath: filepath.Join(t.TempDir(), "source.mp3"),
		Status:     model.IngestJobStatusFailed,
		Error:      "boom",
	})
	if err != nil {
		t.Fatalf("create ingest job: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/ingest/jobs/"+string(created.JobID), nil)
	req.Header.Set("X-Admin-Token", "secret")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := doRequest(t, app.handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp adminIngestJobResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.JobID != "job_lookup" || resp.AssetID != "asset_lookup" || resp.Status != "failed" || resp.Error != "boom" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func newUploadRequest(t *testing.T, path string, artistID string, payeeID string, title string, priceMSat string, assetID string, payload []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	mustWriteField(t, writer, "artist_id", artistID)
	mustWriteField(t, writer, "payee_id", payeeID)
	mustWriteField(t, writer, "title", title)
	mustWriteField(t, writer, "price_msat", priceMSat)
	if strings.TrimSpace(assetID) != "" {
		mustWriteField(t, writer, "asset_id", assetID)
	}
	part, err := writer.CreateFormFile("audio", "track.mp3")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.RemoteAddr = "127.0.0.1:1234"
	return req
}

func mustWriteField(t *testing.T, writer *multipart.Writer, key string, value string) {
	t.Helper()
	if err := writer.WriteField(key, value); err != nil {
		t.Fatalf("write field %s: %v", key, err)
	}
}

func seedArtistAndPayee(t *testing.T, db *sql.DB) {
	t.Helper()
	artistsRepo := storesqlite.NewArtistsRepo(db)
	payeesRepo := storesqlite.NewPayeesRepo(db)
	ctx := context.Background()
	_, err := artistsRepo.CreateArtist(ctx, repo.CreateArtistParams{
		ArtistID:    model.ArtistID("artist_upload"),
		PubKeyHex:   strings.Repeat("a", 64),
		Handle:      "artist-upload",
		DisplayName: "Artist Upload",
	})
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	_, err = payeesRepo.CreatePayee(ctx, repo.CreatePayeeParams{
		PayeeID:          model.PayeeID("payee_upload"),
		ArtistID:         model.ArtistID("artist_upload"),
		FAPPublicBaseURL: "http://localhost:18081",
		FAPPayeeID:       "fap_payee_upload",
	})
	if err != nil {
		t.Fatalf("create payee: %v", err)
	}
}

func fetchIngestJob(t *testing.T, db *sql.DB, jobID string) model.IngestJob {
	t.Helper()
	job, err := storesqlite.NewIngestJobsRepo(db).GetIngestJob(context.Background(), model.IngestJobID(jobID))
	if err != nil {
		t.Fatalf("get ingest job: %v", err)
	}
	return job
}
