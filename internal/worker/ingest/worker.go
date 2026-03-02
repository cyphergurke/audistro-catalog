package ingest

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"audistro-catalog/internal/model"
	ingestsvc "audistro-catalog/internal/service/ingest"
)

const defaultProviderDataPath = "/mnt/providers/eu_1"

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Logger interface {
	Printf(format string, args ...any)
}

type ProviderTarget struct {
	Name            string
	PublicBaseURL   string
	InternalBaseURL string
	DataPathMount   string
}

type PublishTargetFailure struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

type PublishReport struct {
	PublishedTargets []string               `json:"published_targets,omitempty"`
	FailedTargets    []PublishTargetFailure `json:"failed_targets,omitempty"`
}

type Dependencies struct {
	IngestService            *ingestsvc.Service
	Runner                   CommandRunner
	HTTPClient               HTTPDoer
	Logger                   Logger
	StoragePath              string
	ProviderDataPath         string
	ProviderInternalBaseURL  string
	ProviderTargets          []ProviderTarget
	FAPInternalBaseURL       string
	FAPPublicBaseURL         string
	FAPAdminToken            string
	WorkerPollInterval       time.Duration
	StaleProcessingThreshold time.Duration
}

type Worker struct {
	ingestService            *ingestsvc.Service
	runner                   CommandRunner
	httpClient               HTTPDoer
	logger                   Logger
	storagePath              string
	providerTargets          []ProviderTarget
	fapInternalBaseURL       string
	fapPublicBaseURL         string
	fapAdminToken            string
	workerPollInterval       time.Duration
	staleProcessingThreshold time.Duration
	nowFn                    func() time.Time
}

func NewWorker(deps Dependencies) (*Worker, error) {
	if deps.IngestService == nil {
		return nil, errors.New("ingest service is required")
	}
	if deps.Runner == nil {
		return nil, errors.New("command runner is required")
	}
	if deps.HTTPClient == nil {
		return nil, errors.New("http client is required")
	}
	if strings.TrimSpace(deps.StoragePath) == "" {
		return nil, errors.New("storage path is required")
	}
	providerTargets := normalizeProviderTargets(deps.ProviderTargets)
	if len(providerTargets) == 0 {
		providerDataPath := strings.TrimSpace(deps.ProviderDataPath)
		if providerDataPath == "" {
			providerDataPath = defaultProviderDataPath
		}
		providerInternalBaseURL := strings.TrimRight(strings.TrimSpace(deps.ProviderInternalBaseURL), "/")
		if providerInternalBaseURL == "" {
			return nil, errors.New("provider internal base url is required")
		}
		providerTargets = []ProviderTarget{{
			Name:            "primary",
			PublicBaseURL:   "",
			InternalBaseURL: providerInternalBaseURL,
			DataPathMount:   providerDataPath,
		}}
	}
	fapInternalBaseURL := strings.TrimRight(strings.TrimSpace(deps.FAPInternalBaseURL), "/")
	if fapInternalBaseURL == "" {
		return nil, errors.New("fap internal base url is required")
	}
	fapPublicBaseURL := strings.TrimRight(strings.TrimSpace(deps.FAPPublicBaseURL), "/")
	if fapPublicBaseURL == "" {
		return nil, errors.New("fap public base url is required")
	}
	fapAdminToken := strings.TrimSpace(deps.FAPAdminToken)
	if fapAdminToken == "" {
		return nil, errors.New("fap admin token is required")
	}
	workerPollInterval := deps.WorkerPollInterval
	if workerPollInterval <= 0 {
		workerPollInterval = 2 * time.Second
	}
	staleProcessingThreshold := deps.StaleProcessingThreshold
	if staleProcessingThreshold <= 0 {
		staleProcessingThreshold = 5 * time.Minute
	}
	return &Worker{
		ingestService:            deps.IngestService,
		runner:                   deps.Runner,
		httpClient:               deps.HTTPClient,
		logger:                   deps.Logger,
		storagePath:              strings.TrimSpace(deps.StoragePath),
		providerTargets:          providerTargets,
		fapInternalBaseURL:       fapInternalBaseURL,
		fapPublicBaseURL:         fapPublicBaseURL,
		fapAdminToken:            fapAdminToken,
		workerPollInterval:       workerPollInterval,
		staleProcessingThreshold: staleProcessingThreshold,
		nowFn:                    time.Now,
	}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.workerPollInterval)
	defer ticker.Stop()

	for {
		processed, err := w.ProcessNext(ctx)
		if err != nil {
			return err
		}
		if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (w *Worker) ProcessNext(ctx context.Context) (bool, error) {
	job, err := w.ingestService.ClaimNextJob(ctx, w.nowFn().Add(-w.staleProcessingThreshold))
	if err != nil {
		if errors.Is(err, ingestsvc.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("claim next ingest job: %w", err)
	}

	if w.logger != nil {
		w.logger.Printf("processing ingest job job_id=%s asset_id=%s", job.JobID, job.AssetID)
	}

	reportJSON, err := w.processJob(ctx, job)
	if err != nil {
		report, reportErr := publishReportFromError(err)
		if reportErr != nil && w.logger != nil {
			w.logger.Printf("ingest job publish report parse failed job_id=%s asset_id=%s error=%v", job.JobID, job.AssetID, reportErr)
		}
		_ = w.ingestService.MarkJobFailed(ctx, string(job.JobID), err, report)
		if w.logger != nil {
			w.logger.Printf("ingest job failed job_id=%s asset_id=%s error=%v", job.JobID, job.AssetID, err)
		}
		return true, nil
	}

	if err := w.ingestService.MarkJobPublished(ctx, string(job.JobID), reportJSON); err != nil {
		return true, fmt.Errorf("mark ingest job published: %w", err)
	}
	if w.logger != nil {
		w.logger.Printf("ingest job published job_id=%s asset_id=%s", job.JobID, job.AssetID)
	}
	return true, nil
}

func (w *Worker) processJob(ctx context.Context, job model.IngestJob) (string, error) {
	buildDir := filepath.Join(w.storagePath, "build", string(job.AssetID))
	if err := os.RemoveAll(buildDir); err != nil {
		return "", fmt.Errorf("clean build dir: %w", err)
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return "", fmt.Errorf("create build dir: %w", err)
	}

	playlistPath := filepath.Join(buildDir, "master.m3u8")
	segmentPattern := filepath.Join(buildDir, "seg_%04d.ts")
	keyPath := filepath.Join(buildDir, "enc.key")
	keyInfoPath := filepath.Join(buildDir, "enc.keyinfo")
	publicKeyURI := buildPublicKeyURI(w.fapPublicBaseURL, string(job.AssetID))
	if err := w.writePackagingKeyFiles(ctx, string(job.AssetID), keyPath, keyInfoPath, publicKeyURI); err != nil {
		return "", fmt.Errorf("prepare packaging key files: %w", err)
	}
	ffmpegArgs := []string{
		"-y",
		"-i", job.SourcePath,
		"-c:a", "aac",
		"-b:a", "128k",
		"-ac", "2",
		"-ar", "44100",
		"-f", "hls",
		"-hls_time", "4",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", segmentPattern,
		"-hls_key_info_file", keyInfoPath,
		playlistPath,
	}
	if err := w.runner.Run(ctx, "ffmpeg", ffmpegArgs...); err != nil {
		return "", fmt.Errorf("ffmpeg packaging failed: %w", err)
	}

	durationMS, err := parsePlaylistDurationMS(playlistPath)
	if err != nil {
		return "", fmt.Errorf("parse playlist duration: %w", err)
	}
	if durationMS <= 0 {
		return "", errors.New("playlist duration is zero")
	}

	report, publishErr := w.publishToTargets(ctx, buildDir, string(job.AssetID))
	reportJSON, err := marshalPublishReport(report)
	if err != nil {
		return "", fmt.Errorf("marshal publish report: %w", err)
	}
	if publishErr != nil && len(report.PublishedTargets) == 0 {
		return "", newPublishError(publishErr, reportJSON)
	}

	if err := w.ingestService.UpsertPublishedAsset(ctx, ingestsvc.PublishedAssetInput{
		AssetID:      string(job.AssetID),
		ArtistID:     string(job.ArtistID),
		PayeeID:      string(job.PayeeID),
		Title:        job.Title,
		PriceMSat:    job.PriceMSat,
		DurationMS:   durationMS,
		HLSMasterURL: w.ingestService.PublicHLSMasterURL(string(job.AssetID)),
		ContentID:    string(job.AssetID),
	}); err != nil {
		return "", newPublishError(fmt.Errorf("upsert published asset: %w", err), reportJSON)
	}

	if publishErr != nil && w.logger != nil {
		w.logger.Printf("ingest job partial publish asset_id=%s report=%s", job.AssetID, reportJSON)
	}

	return reportJSON, nil
}

func (w *Worker) publishToTargets(ctx context.Context, buildDir string, assetID string) (PublishReport, error) {
	skip := map[string]struct{}{
		"enc.key":     {},
		"enc.keyinfo": {},
	}
	report := PublishReport{
		PublishedTargets: make([]string, 0, len(w.providerTargets)),
		FailedTargets:    make([]PublishTargetFailure, 0),
	}
	var failures []string
	for _, target := range w.providerTargets {
		if err := publishDirectory(buildDir, filepath.Join(target.DataPathMount, "assets", assetID), skip); err != nil {
			failures = append(failures, fmt.Sprintf("%s: publish asset dir: %v", target.Name, err))
			report.FailedTargets = append(report.FailedTargets, PublishTargetFailure{Name: target.Name, Error: "publish asset dir: " + err.Error()})
			continue
		}
		if err := w.postJSON(ctx, target.InternalBaseURL, "/internal/rescan", nil); err != nil {
			failures = append(failures, fmt.Sprintf("%s: provider rescan failed: %v", target.Name, err))
			report.FailedTargets = append(report.FailedTargets, PublishTargetFailure{Name: target.Name, Error: "provider rescan failed: " + err.Error()})
			continue
		}
		if err := w.postJSON(ctx, target.InternalBaseURL, "/internal/announce", map[string]string{"asset_id": assetID}); err != nil {
			failures = append(failures, fmt.Sprintf("%s: provider announce failed: %v", target.Name, err))
			report.FailedTargets = append(report.FailedTargets, PublishTargetFailure{Name: target.Name, Error: "provider announce failed: " + err.Error()})
			continue
		}
		report.PublishedTargets = append(report.PublishedTargets, target.Name)
	}
	if len(report.PublishedTargets) == 0 && len(failures) > 0 {
		return report, errors.New(strings.Join(failures, "; "))
	}
	if len(failures) > 0 {
		return report, errors.New(strings.Join(failures, "; "))
	}
	return report, nil
}

func (w *Worker) writePackagingKeyFiles(ctx context.Context, assetID string, keyPath string, keyInfoPath string, publicKeyURI string) error {
	keyBytes, err := w.fetchPackagingKey(ctx, assetID)
	if err != nil {
		return err
	}
	if len(keyBytes) != 16 {
		return fmt.Errorf("unexpected packaging key length %d", len(keyBytes))
	}
	if err := os.WriteFile(keyPath, keyBytes, 0o600); err != nil {
		return fmt.Errorf("write key file: %w", err)
	}
	ivHex := deterministicIVHex(assetID)
	keyInfo := publicKeyURI + "\n" + keyPath + "\n" + ivHex + "\n"
	if err := os.WriteFile(keyInfoPath, []byte(keyInfo), 0o600); err != nil {
		return fmt.Errorf("write keyinfo file: %w", err)
	}
	return nil
}

func (w *Worker) fetchPackagingKey(ctx context.Context, assetID string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.fapInternalBaseURL+"/internal/assets/"+assetID+"/packaging-key", nil)
	if err != nil {
		return nil, fmt.Errorf("build packaging key request: %w", err)
	}
	req.Header.Set("X-Admin-Token", w.fapAdminToken)
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform packaging key request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("packaging key status %d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	keyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 32))
	if err != nil {
		return nil, fmt.Errorf("read packaging key response: %w", err)
	}
	return keyBytes, nil
}

func (w *Worker) postJSON(ctx context.Context, baseURL string, path string, payload any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("unexpected status %d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	return nil
}

func normalizeProviderTargets(targets []ProviderTarget) []ProviderTarget {
	normalized := make([]ProviderTarget, 0, len(targets))
	for _, target := range targets {
		name := strings.TrimSpace(target.Name)
		publicBaseURL := strings.TrimRight(strings.TrimSpace(target.PublicBaseURL), "/")
		internalBaseURL := strings.TrimRight(strings.TrimSpace(target.InternalBaseURL), "/")
		dataPathMount := strings.TrimRight(strings.TrimSpace(target.DataPathMount), "/")
		if name == "" || internalBaseURL == "" || dataPathMount == "" {
			continue
		}
		normalized = append(normalized, ProviderTarget{
			Name:            name,
			PublicBaseURL:   publicBaseURL,
			InternalBaseURL: internalBaseURL,
			DataPathMount:   dataPathMount,
		})
	}
	return normalized
}

func marshalPublishReport(report PublishReport) (string, error) {
	encoded, err := json.Marshal(report)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

type publishError struct {
	err    error
	report string
}

func (e *publishError) Error() string {
	return e.err.Error()
}

func (e *publishError) Unwrap() error {
	return e.err
}

func newPublishError(err error, report string) error {
	if err == nil {
		return nil
	}
	return &publishError{err: err, report: report}
}

func publishReportFromError(err error) (string, error) {
	var target *publishError
	if errors.As(err, &target) {
		return target.report, nil
	}
	return "", errors.New("publish report not attached")
}

func parsePlaylistDurationMS(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var totalMS float64
	var sawSegment bool
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "#EXTINF:") {
			continue
		}
		value := strings.TrimPrefix(line, "#EXTINF:")
		comma := strings.IndexByte(value, ',')
		if comma >= 0 {
			value = value[:comma]
		}
		seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return 0, fmt.Errorf("parse extinf %q: %w", line, err)
		}
		sawSegment = true
		totalMS += seconds * 1000
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if !sawSegment {
		return 0, nil
	}
	rounded := int64(totalMS + 0.5)
	if rounded == 0 && totalMS > 0 {
		return 1, nil
	}
	return rounded, nil
}

func publishDirectory(srcDir string, dstDir string, skip map[string]struct{}) error {
	if err := os.RemoveAll(dstDir); err != nil {
		return err
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if _, ok := skip[entry.Name()]; ok {
			continue
		}
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())
		if entry.IsDir() {
			if err := publishDirectory(srcPath, dstPath, skip); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func buildPublicKeyURI(baseURL string, assetID string) string {
	return strings.TrimRight(baseURL, "/") + "/hls/" + assetID + "/key"
}

func deterministicIVHex(assetID string) string {
	sum := sha256.Sum256([]byte("hls-iv|" + assetID))
	return hex.EncodeToString(sum[:16])
}

func copyFile(srcPath string, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	return nil
}
