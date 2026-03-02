package ingest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"audistro-catalog/internal/model"
	"audistro-catalog/internal/store/repo"
)

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrInvalidInput  = errors.New("invalid_input")
	ErrAdminDisabled = errors.New("admin_disabled")
	ErrUnauthorized  = errors.New("unauthorized")
)

type Service struct {
	artistsRepo           repo.ArtistsRepository
	payeesRepo            repo.PayeesRepository
	assetsRepo            repo.AssetsRepository
	ingestJobsRepo        repo.IngestJobsRepository
	storagePath           string
	providerPublicBaseURL string
	nowFn                 func() time.Time
}

type QueueUploadInput struct {
	ArtistID  string
	PayeeID   string
	Title     string
	PriceMSat int64
	AssetID   string
	Source    io.Reader
}

type PublishedAssetInput struct {
	AssetID      string
	ArtistID     string
	PayeeID      string
	Title        string
	PriceMSat    int64
	DurationMS   int64
	HLSMasterURL string
	ContentID    string
}

type Job struct {
	JobID         string `json:"job_id"`
	AssetID       string `json:"asset_id"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
	PublishReport string `json:"publish_report,omitempty"`
	CreatedAt     int64  `json:"created_at,omitempty"`
	UpdatedAt     int64  `json:"updated_at,omitempty"`
}

func NewService(
	artistsRepo repo.ArtistsRepository,
	payeesRepo repo.PayeesRepository,
	assetsRepo repo.AssetsRepository,
	ingestJobsRepo repo.IngestJobsRepository,
	storagePath string,
	providerPublicBaseURL string,
) (*Service, error) {
	if artistsRepo == nil {
		return nil, errors.New("artists repo is required")
	}
	if payeesRepo == nil {
		return nil, errors.New("payees repo is required")
	}
	if assetsRepo == nil {
		return nil, errors.New("assets repo is required")
	}
	if ingestJobsRepo == nil {
		return nil, errors.New("ingest jobs repo is required")
	}
	storagePath = strings.TrimSpace(storagePath)
	if storagePath == "" {
		return nil, errors.New("storage path is required")
	}

	return &Service{
		artistsRepo:           artistsRepo,
		payeesRepo:            payeesRepo,
		assetsRepo:            assetsRepo,
		ingestJobsRepo:        ingestJobsRepo,
		storagePath:           storagePath,
		providerPublicBaseURL: strings.TrimRight(strings.TrimSpace(providerPublicBaseURL), "/"),
		nowFn:                 time.Now,
	}, nil
}

func (s *Service) QueueUpload(ctx context.Context, input QueueUploadInput) (Job, error) {
	artistID := strings.TrimSpace(input.ArtistID)
	payeeID := strings.TrimSpace(input.PayeeID)
	title := strings.TrimSpace(input.Title)
	assetID := strings.TrimSpace(input.AssetID)

	if !idPattern.MatchString(artistID) {
		return Job{}, fmt.Errorf("artist_id: %w", ErrInvalidInput)
	}
	if !idPattern.MatchString(payeeID) {
		return Job{}, fmt.Errorf("payee_id: %w", ErrInvalidInput)
	}
	if assetID != "" && !idPattern.MatchString(assetID) {
		return Job{}, fmt.Errorf("asset_id: %w", ErrInvalidInput)
	}
	if title == "" {
		return Job{}, fmt.Errorf("title: %w", ErrInvalidInput)
	}
	if input.PriceMSat < 0 {
		return Job{}, fmt.Errorf("price_msat: %w", ErrInvalidInput)
	}
	if input.Source == nil {
		return Job{}, fmt.Errorf("audio: %w", ErrInvalidInput)
	}

	artist, err := s.artistsRepo.GetArtistByID(ctx, model.ArtistID(artistID))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return Job{}, fmt.Errorf("artist lookup: %w", ErrNotFound)
		}
		return Job{}, fmt.Errorf("artist lookup: %w", err)
	}
	payee, err := s.payeesRepo.GetPayee(ctx, model.PayeeID(payeeID))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return Job{}, fmt.Errorf("payee lookup: %w", ErrNotFound)
		}
		return Job{}, fmt.Errorf("payee lookup: %w", err)
	}
	if payee.ArtistID != artist.ArtistID {
		return Job{}, fmt.Errorf("payee artist mismatch: %w", ErrInvalidInput)
	}

	tmpDir := filepath.Join(s.storagePath, "uploads", ".tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return Job{}, fmt.Errorf("create temp upload dir: %w", err)
	}
	file, err := os.CreateTemp(tmpDir, "upload-*.mp3")
	if err != nil {
		return Job{}, fmt.Errorf("create temp upload file: %w", err)
	}
	tmpPath := file.Name()
	defer func() {
		_ = file.Close()
	}()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()
	_, _ = io.WriteString(hasher, artistID+"\n"+payeeID+"\n"+title+"\n"+strconv.FormatInt(input.PriceMSat, 10)+"\n")
	if _, err := io.Copy(io.MultiWriter(file, hasher), input.Source); err != nil {
		return Job{}, fmt.Errorf("write upload: %w", err)
	}
	if err := file.Close(); err != nil {
		return Job{}, fmt.Errorf("close upload: %w", err)
	}

	digest := hex.EncodeToString(hasher.Sum(nil))
	if assetID == "" {
		assetID = "au_" + digest[:20]
	}
	jobID, err := generateJobID()
	if err != nil {
		return Job{}, fmt.Errorf("generate job id: %w", err)
	}

	uploadDir := filepath.Join(s.storagePath, "uploads", assetID)
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return Job{}, fmt.Errorf("create upload dir: %w", err)
	}
	sourcePath := filepath.Join(uploadDir, "source.mp3")
	if err := os.Rename(tmpPath, sourcePath); err != nil {
		return Job{}, fmt.Errorf("store upload: %w", err)
	}
	cleanupTemp = false

	job, err := s.ingestJobsRepo.CreateIngestJob(ctx, repo.CreateIngestJobParams{
		JobID:      model.IngestJobID(jobID),
		AssetID:    model.AssetID(assetID),
		ArtistID:   artist.ArtistID,
		PayeeID:    payee.PayeeID,
		Title:      title,
		PriceMSat:  input.PriceMSat,
		SourcePath: sourcePath,
		Status:     model.IngestJobStatusQueued,
	})
	if err != nil {
		return Job{}, fmt.Errorf("create ingest job: %w", err)
	}

	return toJob(job), nil
}

func (s *Service) GetJob(ctx context.Context, jobID string) (Job, error) {
	if !idPattern.MatchString(strings.TrimSpace(jobID)) {
		return Job{}, fmt.Errorf("job_id: %w", ErrInvalidInput)
	}
	job, err := s.ingestJobsRepo.GetIngestJob(ctx, model.IngestJobID(jobID))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return Job{}, fmt.Errorf("get ingest job: %w", ErrNotFound)
		}
		return Job{}, fmt.Errorf("get ingest job: %w", err)
	}
	return toJob(job), nil
}

func (s *Service) ClaimNextJob(ctx context.Context, staleBefore time.Time) (model.IngestJob, error) {
	job, err := s.ingestJobsRepo.ClaimNextIngestJob(ctx, staleBefore.Unix())
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return model.IngestJob{}, fmt.Errorf("claim ingest job: %w", ErrNotFound)
		}
		return model.IngestJob{}, fmt.Errorf("claim ingest job: %w", err)
	}
	return job, nil
}

func (s *Service) MarkJobPublished(ctx context.Context, jobID string, publishReport string) error {
	_, err := s.ingestJobsRepo.UpdateIngestJobStatus(ctx, repo.UpdateIngestJobStatusParams{
		JobID:         model.IngestJobID(jobID),
		Status:        model.IngestJobStatusPublished,
		PublishReport: publishReport,
	})
	if err != nil {
		return fmt.Errorf("mark ingest job published: %w", err)
	}
	return nil
}

func (s *Service) MarkJobFailed(ctx context.Context, jobID string, failure error, publishReport string) error {
	message := strings.TrimSpace(errorString(failure))
	_, err := s.ingestJobsRepo.UpdateIngestJobStatus(ctx, repo.UpdateIngestJobStatusParams{
		JobID:         model.IngestJobID(jobID),
		Status:        model.IngestJobStatusFailed,
		Error:         truncate(message, 1024),
		PublishReport: publishReport,
	})
	if err != nil {
		return fmt.Errorf("mark ingest job failed: %w", err)
	}
	return nil
}

func (s *Service) UpsertPublishedAsset(ctx context.Context, input PublishedAssetInput) error {
	_, err := s.assetsRepo.UpsertAsset(ctx, repo.UpsertAssetParams{
		AssetID:      model.AssetID(strings.TrimSpace(input.AssetID)),
		ArtistID:     model.ArtistID(strings.TrimSpace(input.ArtistID)),
		PayeeID:      model.PayeeID(strings.TrimSpace(input.PayeeID)),
		Title:        strings.TrimSpace(input.Title),
		DurationMS:   input.DurationMS,
		ContentID:    strings.TrimSpace(input.ContentID),
		HLSMasterURL: strings.TrimSpace(input.HLSMasterURL),
		PriceMSat:    input.PriceMSat,
	})
	if err != nil {
		return fmt.Errorf("upsert asset: %w", err)
	}
	return nil
}

func (s *Service) PublicHLSMasterURL(assetID string) string {
	if s.providerPublicBaseURL == "" {
		return ""
	}
	return s.providerPublicBaseURL + "/assets/" + strings.TrimSpace(assetID) + "/master.m3u8"
}

func toJob(job model.IngestJob) Job {
	return Job{
		JobID:         string(job.JobID),
		AssetID:       string(job.AssetID),
		Status:        string(job.Status),
		Error:         job.Error,
		PublishReport: job.PublishReport,
		CreatedAt:     job.CreatedAt,
		UpdatedAt:     job.UpdatedAt,
	}
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func generateJobID() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "job_" + hex.EncodeToString(raw[:]), nil
}
