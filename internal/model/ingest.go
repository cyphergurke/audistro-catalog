package model

type IngestJobStatus string

const (
	IngestJobStatusQueued     IngestJobStatus = "queued"
	IngestJobStatusProcessing IngestJobStatus = "processing"
	IngestJobStatusPublished  IngestJobStatus = "published"
	IngestJobStatusFailed     IngestJobStatus = "failed"
)

type IngestJobID string

type IngestJob struct {
	JobID         IngestJobID
	AssetID       AssetID
	ArtistID      ArtistID
	PayeeID       PayeeID
	Title         string
	PriceMSat     int64
	SourcePath    string
	Status        IngestJobStatus
	Error         string
	PublishReport string
	CreatedAt     int64
	UpdatedAt     int64
}
