package repo

import (
	"context"

	"github.com/cyphergurke/audistro-catalog/internal/model"
)

type AddProviderHintParams struct {
	HintID    model.ProviderHintID
	AssetID   model.AssetID
	Transport string
	BaseURL   string
	Priority  int64
}

type ProviderHintsRepository interface {
	AddHint(ctx context.Context, params AddProviderHintParams) (model.ProviderHint, error)
	ListHintsByAsset(ctx context.Context, assetID model.AssetID) ([]model.ProviderHint, error)
}
