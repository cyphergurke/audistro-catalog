package repo

import (
	"context"

	"audistro-catalog/internal/model"
)

type CreateIngestJobParams struct {
	JobID         model.IngestJobID
	AssetID       model.AssetID
	ArtistID      model.ArtistID
	PayeeID       model.PayeeID
	Title         string
	PriceMSat     int64
	SourcePath    string
	Status        model.IngestJobStatus
	Error         string
	PublishReport string
}

type UpdateIngestJobStatusParams struct {
	JobID         model.IngestJobID
	Status        model.IngestJobStatus
	Error         string
	PublishReport string
	UpdatedAt     int64
}

type IngestJobsRepository interface {
	CreateIngestJob(ctx context.Context, params CreateIngestJobParams) (model.IngestJob, error)
	GetIngestJob(ctx context.Context, jobID model.IngestJobID) (model.IngestJob, error)
	ClaimNextIngestJob(ctx context.Context, staleBefore int64) (model.IngestJob, error)
	UpdateIngestJobStatus(ctx context.Context, params UpdateIngestJobStatusParams) (model.IngestJob, error)
}
