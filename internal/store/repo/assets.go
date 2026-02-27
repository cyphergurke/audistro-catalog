package repo

import (
	"context"

	"github.com/cyphergurke/audistro-catalog/internal/model"
)

type CreateAssetParams struct {
	AssetID       model.AssetID
	ArtistID      model.ArtistID
	PayeeID       model.PayeeID
	Title         string
	DurationMS    int64
	ContentID     string
	HLSMasterURL  string
	PreviewHLSURL string
	PriceMSat     int64
}

type AssetsRepository interface {
	CreateAsset(ctx context.Context, params CreateAssetParams) (model.Asset, error)
	GetAsset(ctx context.Context, assetID model.AssetID) (model.Asset, error)
	ListAssetsByArtist(ctx context.Context, artistID model.ArtistID) ([]model.Asset, error)
}
