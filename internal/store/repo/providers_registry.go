package repo

import (
	"context"

	"github.com/cyphergurke/audistro-catalog/internal/model"
)

type UpsertProviderParams struct {
	ProviderID string
	PublicKey  string
	Transport  string
	BaseURL    string
	Region     string
	Status     string
}

type UpsertProviderAssetAnnouncementParams struct {
	ProviderID string
	AssetID    model.AssetID
	Transport  string
	BaseURL    string
	Priority   int64
	ExpiresAt  int64
	LastSeenAt int64
	Nonce      string
}

type ProviderRegistryRepository interface {
	GetProvider(ctx context.Context, providerID string) (model.Provider, error)
	UpsertProvider(ctx context.Context, params UpsertProviderParams) (model.Provider, error)
	GetProviderAssetAnnouncement(ctx context.Context, providerID string, assetID model.AssetID) (model.ProviderAssetAnnouncement, error)
	UpsertProviderAssetAnnouncement(ctx context.Context, params UpsertProviderAssetAnnouncementParams) (model.ProviderAssetAnnouncement, error)
	ListActiveProvidersByAsset(ctx context.Context, assetID model.AssetID, nowUnix int64, limit int) ([]model.AssetProviderHint, error)
	CountActiveProvidersByAsset(ctx context.Context, assetID model.AssetID, nowUnix int64) (int64, error)
	DeleteExpiredProviderAssets(ctx context.Context, nowUnix int64) (int64, error)
}
