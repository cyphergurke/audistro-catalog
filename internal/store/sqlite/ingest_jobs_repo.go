package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"audistro-catalog/internal/model"
	"audistro-catalog/internal/store/repo"
)

var _ repo.IngestJobsRepository = (*IngestJobsRepo)(nil)

type IngestJobsRepo struct {
	db *sql.DB
}

func NewIngestJobsRepo(db *sql.DB) *IngestJobsRepo {
	return &IngestJobsRepo{db: db}
}

func (r *IngestJobsRepo) CreateIngestJob(ctx context.Context, params repo.CreateIngestJobParams) (model.IngestJob, error) {
	now := time.Now().Unix()
	job := model.IngestJob{
		JobID:         params.JobID,
		AssetID:       params.AssetID,
		ArtistID:      params.ArtistID,
		PayeeID:       params.PayeeID,
		Title:         params.Title,
		PriceMSat:     params.PriceMSat,
		SourcePath:    params.SourcePath,
		Status:        params.Status,
		Error:         params.Error,
		PublishReport: params.PublishReport,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ingest_jobs(
			job_id, asset_id, artist_id, payee_id, title, price_msat, source_path,
			status, error, publish_report, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, string(job.JobID), string(job.AssetID), string(job.ArtistID), string(job.PayeeID), job.Title,
		job.PriceMSat, job.SourcePath, string(job.Status), nullableString(job.Error), nullableString(job.PublishReport), job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return model.IngestJob{}, fmt.Errorf("create ingest job: %w", err)
	}

	return job, nil
}

func (r *IngestJobsRepo) GetIngestJob(ctx context.Context, jobID model.IngestJobID) (model.IngestJob, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT job_id, asset_id, artist_id, payee_id, title, price_msat, source_path, status, error, publish_report, created_at, updated_at
		FROM ingest_jobs WHERE job_id = ?
	`, string(jobID))
	job, err := scanIngestJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.IngestJob{}, fmt.Errorf("get ingest job: %w", repo.ErrNotFound)
		}
		return model.IngestJob{}, fmt.Errorf("get ingest job: %w", err)
	}
	return job, nil
}

func (r *IngestJobsRepo) ClaimNextIngestJob(ctx context.Context, staleBefore int64) (model.IngestJob, error) {
	for {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return model.IngestJob{}, fmt.Errorf("begin claim ingest job: %w", err)
		}

		row := tx.QueryRowContext(ctx, `
			SELECT job_id, asset_id, artist_id, payee_id, title, price_msat, source_path, status, error, publish_report, created_at, updated_at
			FROM ingest_jobs
			WHERE status = 'queued' OR (status = 'processing' AND updated_at <= ?)
			ORDER BY created_at ASC
			LIMIT 1
		`, staleBefore)
		job, scanErr := scanIngestJob(row)
		if scanErr != nil {
			_ = tx.Rollback()
			if errors.Is(scanErr, sql.ErrNoRows) {
				return model.IngestJob{}, fmt.Errorf("claim ingest job: %w", repo.ErrNotFound)
			}
			return model.IngestJob{}, fmt.Errorf("claim ingest job lookup: %w", scanErr)
		}

		now := time.Now().Unix()
		result, err := tx.ExecContext(ctx, `
			UPDATE ingest_jobs
			SET status = 'processing', error = NULL, publish_report = NULL, updated_at = ?
			WHERE job_id = ? AND (status = 'queued' OR (status = 'processing' AND updated_at <= ?))
		`, now, string(job.JobID), staleBefore)
		if err != nil {
			_ = tx.Rollback()
			return model.IngestJob{}, fmt.Errorf("claim ingest job update: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			_ = tx.Rollback()
			return model.IngestJob{}, fmt.Errorf("claim ingest job rows affected: %w", err)
		}
		if affected == 0 {
			_ = tx.Rollback()
			continue
		}
		if err := tx.Commit(); err != nil {
			return model.IngestJob{}, fmt.Errorf("commit claim ingest job: %w", err)
		}
		job.Status = model.IngestJobStatusProcessing
		job.Error = ""
		job.PublishReport = ""
		job.UpdatedAt = now
		return job, nil
	}
}

func (r *IngestJobsRepo) UpdateIngestJobStatus(ctx context.Context, params repo.UpdateIngestJobStatusParams) (model.IngestJob, error) {
	updatedAt := params.UpdatedAt
	if updatedAt <= 0 {
		updatedAt = time.Now().Unix()
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE ingest_jobs
		SET status = ?, error = ?, publish_report = ?, updated_at = ?
		WHERE job_id = ?
	`, string(params.Status), nullableString(params.Error), nullableString(params.PublishReport), updatedAt, string(params.JobID))
	if err != nil {
		return model.IngestJob{}, fmt.Errorf("update ingest job status: %w", err)
	}
	return r.GetIngestJob(ctx, params.JobID)
}

type ingestJobScanner interface {
	Scan(dest ...any) error
}

func scanIngestJob(row ingestJobScanner) (model.IngestJob, error) {
	var job model.IngestJob
	var jobID string
	var assetID string
	var artistID string
	var payeeID string
	var status string
	var errorText sql.NullString
	var publishReport sql.NullString
	if err := row.Scan(
		&jobID,
		&assetID,
		&artistID,
		&payeeID,
		&job.Title,
		&job.PriceMSat,
		&job.SourcePath,
		&status,
		&errorText,
		&publishReport,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		return model.IngestJob{}, err
	}
	job.JobID = model.IngestJobID(jobID)
	job.AssetID = model.AssetID(assetID)
	job.ArtistID = model.ArtistID(artistID)
	job.PayeeID = model.PayeeID(payeeID)
	job.Status = model.IngestJobStatus(status)
	job.Error = errorText.String
	job.PublishReport = publishReport.String
	return job, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
