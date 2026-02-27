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

var _ repo.ProviderRegistryRepository = (*ProviderRegistryRepo)(nil)

type ProviderRegistryRepo struct {
	db *sql.DB
}

func NewProviderRegistryRepo(db *sql.DB) *ProviderRegistryRepo {
	return &ProviderRegistryRepo{db: db}
}

func (r *ProviderRegistryRepo) GetProvider(ctx context.Context, providerID string) (model.Provider, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT provider_id, public_key, transport, base_url, COALESCE(region, ''), status, created_at, updated_at
		FROM providers
		WHERE provider_id = ?
	`)
	if err != nil {
		return model.Provider{}, fmt.Errorf("prepare get provider: %w", err)
	}
	defer stmt.Close()

	provider, err := scanProvider(stmt.QueryRowContext(ctx, providerID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Provider{}, fmt.Errorf("get provider: %w", repo.ErrNotFound)
		}
		return model.Provider{}, fmt.Errorf("get provider: %w", err)
	}

	return provider, nil
}

func (r *ProviderRegistryRepo) UpsertProvider(ctx context.Context, params repo.UpsertProviderParams) (model.Provider, error) {
	now := time.Now().Unix()
	provider := model.Provider{
		ProviderID: params.ProviderID,
		PublicKey:  params.PublicKey,
		Transport:  params.Transport,
		BaseURL:    params.BaseURL,
		Region:     params.Region,
		Status:     params.Status,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if provider.Transport == "" {
		provider.Transport = "https"
	}
	if provider.Status == "" {
		provider.Status = "active"
	}

	stmt, err := r.db.PrepareContext(ctx, `
		INSERT INTO providers(provider_id, public_key, transport, base_url, region, status, created_at, updated_at)
		VALUES(?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?)
		ON CONFLICT(provider_id) DO UPDATE SET
			transport = excluded.transport,
			base_url = excluded.base_url,
			region = excluded.region,
			status = excluded.status,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return model.Provider{}, fmt.Errorf("prepare upsert provider: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(
		ctx,
		provider.ProviderID,
		provider.PublicKey,
		provider.Transport,
		provider.BaseURL,
		provider.Region,
		provider.Status,
		provider.CreatedAt,
		provider.UpdatedAt,
	)
	if err != nil {
		return model.Provider{}, fmt.Errorf("upsert provider: %w", err)
	}

	stored, err := r.GetProvider(ctx, provider.ProviderID)
	if err != nil {
		return model.Provider{}, fmt.Errorf("load provider after upsert: %w", err)
	}
	return stored, nil
}

func (r *ProviderRegistryRepo) GetProviderAssetAnnouncement(ctx context.Context, providerID string, assetID model.AssetID) (model.ProviderAssetAnnouncement, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT provider_id, asset_id, transport, base_url, priority, expires_at, last_seen_at, nonce, created_at, updated_at
		FROM provider_assets
		WHERE provider_id = ? AND asset_id = ?
	`)
	if err != nil {
		return model.ProviderAssetAnnouncement{}, fmt.Errorf("prepare get provider asset announcement: %w", err)
	}
	defer stmt.Close()

	announcement, err := scanProviderAssetAnnouncement(stmt.QueryRowContext(ctx, providerID, string(assetID)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ProviderAssetAnnouncement{}, fmt.Errorf("get provider asset announcement: %w", repo.ErrNotFound)
		}
		return model.ProviderAssetAnnouncement{}, fmt.Errorf("get provider asset announcement: %w", err)
	}
	return announcement, nil
}

func (r *ProviderRegistryRepo) UpsertProviderAssetAnnouncement(ctx context.Context, params repo.UpsertProviderAssetAnnouncementParams) (model.ProviderAssetAnnouncement, error) {
	now := time.Now().Unix()
	announcement := model.ProviderAssetAnnouncement{
		ProviderID: params.ProviderID,
		AssetID:    params.AssetID,
		Transport:  params.Transport,
		BaseURL:    params.BaseURL,
		Priority:   params.Priority,
		ExpiresAt:  params.ExpiresAt,
		LastSeenAt: params.LastSeenAt,
		Nonce:      params.Nonce,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	stmt, err := r.db.PrepareContext(ctx, `
		INSERT INTO provider_assets(provider_id, asset_id, transport, base_url, priority, expires_at, last_seen_at, nonce, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider_id, asset_id) DO UPDATE SET
			transport = excluded.transport,
			base_url = excluded.base_url,
			priority = excluded.priority,
			expires_at = excluded.expires_at,
			last_seen_at = excluded.last_seen_at,
			nonce = excluded.nonce,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return model.ProviderAssetAnnouncement{}, fmt.Errorf("prepare upsert provider asset announcement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(
		ctx,
		announcement.ProviderID,
		string(announcement.AssetID),
		announcement.Transport,
		announcement.BaseURL,
		announcement.Priority,
		announcement.ExpiresAt,
		announcement.LastSeenAt,
		announcement.Nonce,
		announcement.CreatedAt,
		announcement.UpdatedAt,
	)
	if err != nil {
		return model.ProviderAssetAnnouncement{}, fmt.Errorf("upsert provider asset announcement: %w", err)
	}

	stored, err := r.GetProviderAssetAnnouncement(ctx, announcement.ProviderID, announcement.AssetID)
	if err != nil {
		return model.ProviderAssetAnnouncement{}, fmt.Errorf("load provider asset announcement after upsert: %w", err)
	}
	return stored, nil
}

func (r *ProviderRegistryRepo) ListActiveProvidersByAsset(ctx context.Context, assetID model.AssetID, nowUnix int64, limit int) ([]model.AssetProviderHint, error) {
	if limit <= 0 {
		limit = 200
	}
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT pa.provider_id, pa.transport, pa.base_url, pa.priority, pa.expires_at, pa.last_seen_at, pa.updated_at, COALESCE(p.region, '')
		FROM provider_assets pa
		JOIN providers p ON p.provider_id = pa.provider_id
		WHERE pa.asset_id = ? AND pa.expires_at > ? AND p.status = 'active'
		ORDER BY pa.priority DESC, pa.last_seen_at DESC, pa.provider_id ASC
		LIMIT ?
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare list active providers by asset: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, string(assetID), nowUnix, limit)
	if err != nil {
		return nil, fmt.Errorf("list active providers by asset: %w", err)
	}
	defer rows.Close()

	hints := make([]model.AssetProviderHint, 0)
	for rows.Next() {
		var hint model.AssetProviderHint
		if err := rows.Scan(
			&hint.ProviderID,
			&hint.Transport,
			&hint.BaseURL,
			&hint.Priority,
			&hint.ExpiresAt,
			&hint.LastSeenAt,
			&hint.UpdatedAt,
			&hint.Region,
		); err != nil {
			return nil, fmt.Errorf("scan provider asset row: %w", err)
		}
		hints = append(hints, hint)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider asset rows: %w", err)
	}

	return hints, nil
}

func (r *ProviderRegistryRepo) CountActiveProvidersByAsset(ctx context.Context, assetID model.AssetID, nowUnix int64) (int64, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT COUNT(*)
		FROM provider_assets pa
		JOIN providers p ON p.provider_id = pa.provider_id
		WHERE pa.asset_id = ? AND pa.expires_at > ? AND p.status = 'active'
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare count active providers by asset: %w", err)
	}
	defer stmt.Close()

	var count int64
	if err := stmt.QueryRowContext(ctx, string(assetID), nowUnix).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active providers by asset: %w", err)
	}
	return count, nil
}

func scanProvider(row *sql.Row) (model.Provider, error) {
	var provider model.Provider
	if err := row.Scan(
		&provider.ProviderID,
		&provider.PublicKey,
		&provider.Transport,
		&provider.BaseURL,
		&provider.Region,
		&provider.Status,
		&provider.CreatedAt,
		&provider.UpdatedAt,
	); err != nil {
		return model.Provider{}, err
	}
	return provider, nil
}

func scanProviderAssetAnnouncement(row *sql.Row) (model.ProviderAssetAnnouncement, error) {
	var announcement model.ProviderAssetAnnouncement
	var assetID string
	if err := row.Scan(
		&announcement.ProviderID,
		&assetID,
		&announcement.Transport,
		&announcement.BaseURL,
		&announcement.Priority,
		&announcement.ExpiresAt,
		&announcement.LastSeenAt,
		&announcement.Nonce,
		&announcement.CreatedAt,
		&announcement.UpdatedAt,
	); err != nil {
		return model.ProviderAssetAnnouncement{}, err
	}
	announcement.AssetID = model.AssetID(assetID)
	return announcement, nil
}

func (r *ProviderRegistryRepo) DeleteExpiredProviderAssets(ctx context.Context, nowUnix int64) (int64, error) {
	stmt, err := r.db.PrepareContext(ctx, `DELETE FROM provider_assets WHERE expires_at < ?`)
	if err != nil {
		return 0, fmt.Errorf("prepare delete expired provider assets: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, nowUnix)
	if err != nil {
		return 0, fmt.Errorf("delete expired provider assets: %w", err)
	}

	deleted, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete expired provider assets rows affected: %w", err)
	}
	return deleted, nil
}
