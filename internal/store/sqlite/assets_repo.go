package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cyphergurke/audistro-catalog/internal/model"
	"github.com/cyphergurke/audistro-catalog/internal/store/repo"
)

var _ repo.AssetsRepository = (*AssetsRepo)(nil)

type AssetsRepo struct {
	db *sql.DB
}

func NewAssetsRepo(db *sql.DB) *AssetsRepo {
	return &AssetsRepo{db: db}
}

func (r *AssetsRepo) CreateAsset(ctx context.Context, params repo.CreateAssetParams) (model.Asset, error) {
	now := time.Now().Unix()
	asset := model.Asset{
		AssetID:       params.AssetID,
		ArtistID:      params.ArtistID,
		PayeeID:       params.PayeeID,
		Title:         params.Title,
		DurationMS:    params.DurationMS,
		ContentID:     params.ContentID,
		HLSMasterURL:  params.HLSMasterURL,
		PreviewHLSURL: params.PreviewHLSURL,
		PriceMSat:     params.PriceMSat,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	stmt, err := r.db.PrepareContext(ctx, `
		INSERT INTO assets(
			asset_id, artist_id, payee_id, title, duration_ms, content_id, hls_master_url,
			preview_hls_url, price_msat, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return model.Asset{}, fmt.Errorf("prepare create asset: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(
		ctx,
		string(asset.AssetID),
		string(asset.ArtistID),
		string(asset.PayeeID),
		asset.Title,
		asset.DurationMS,
		asset.ContentID,
		asset.HLSMasterURL,
		asset.PreviewHLSURL,
		asset.PriceMSat,
		asset.CreatedAt,
		asset.UpdatedAt,
	)
	if err != nil {
		return model.Asset{}, fmt.Errorf("create asset: %w", err)
	}

	return asset, nil
}

func (r *AssetsRepo) GetAsset(ctx context.Context, assetID model.AssetID) (model.Asset, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT asset_id, artist_id, payee_id, title, duration_ms, content_id, hls_master_url,
			preview_hls_url, price_msat, created_at, updated_at
		FROM assets WHERE asset_id = ?
	`)
	if err != nil {
		return model.Asset{}, fmt.Errorf("prepare get asset: %w", err)
	}
	defer stmt.Close()

	asset, err := scanAsset(stmt.QueryRowContext(ctx, string(assetID)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Asset{}, fmt.Errorf("get asset: %w", repo.ErrNotFound)
		}
		return model.Asset{}, fmt.Errorf("get asset: %w", err)
	}
	return asset, nil
}

func (r *AssetsRepo) ListAssetsByArtist(ctx context.Context, artistID model.ArtistID) ([]model.Asset, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT asset_id, artist_id, payee_id, title, duration_ms, content_id, hls_master_url,
			preview_hls_url, price_msat, created_at, updated_at
		FROM assets WHERE artist_id = ? ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare list assets by artist: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, string(artistID))
	if err != nil {
		return nil, fmt.Errorf("list assets by artist: %w", err)
	}
	defer rows.Close()

	assets := make([]model.Asset, 0)
	for rows.Next() {
		asset, err := scanAssetRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan asset row: %w", err)
		}
		assets = append(assets, asset)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate asset rows: %w", err)
	}

	return assets, nil
}

func scanAsset(row *sql.Row) (model.Asset, error) {
	var asset model.Asset
	var assetIDRaw string
	var artistIDRaw string
	var payeeIDRaw string
	if err := row.Scan(
		&assetIDRaw,
		&artistIDRaw,
		&payeeIDRaw,
		&asset.Title,
		&asset.DurationMS,
		&asset.ContentID,
		&asset.HLSMasterURL,
		&asset.PreviewHLSURL,
		&asset.PriceMSat,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	); err != nil {
		return model.Asset{}, err
	}
	asset.AssetID = model.AssetID(assetIDRaw)
	asset.ArtistID = model.ArtistID(artistIDRaw)
	asset.PayeeID = model.PayeeID(payeeIDRaw)
	return asset, nil
}

func scanAssetRows(rows *sql.Rows) (model.Asset, error) {
	var asset model.Asset
	var assetIDRaw string
	var artistIDRaw string
	var payeeIDRaw string
	if err := rows.Scan(
		&assetIDRaw,
		&artistIDRaw,
		&payeeIDRaw,
		&asset.Title,
		&asset.DurationMS,
		&asset.ContentID,
		&asset.HLSMasterURL,
		&asset.PreviewHLSURL,
		&asset.PriceMSat,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	); err != nil {
		return model.Asset{}, err
	}
	asset.AssetID = model.AssetID(assetIDRaw)
	asset.ArtistID = model.ArtistID(artistIDRaw)
	asset.PayeeID = model.PayeeID(payeeIDRaw)
	return asset, nil
}
