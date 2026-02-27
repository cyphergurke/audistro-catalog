package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"audistro-catalog/internal/model"
	"audistro-catalog/internal/store/repo"
)

var _ repo.ProviderHintsRepository = (*ProviderHintsRepo)(nil)

type ProviderHintsRepo struct {
	db *sql.DB
}

func NewProviderHintsRepo(db *sql.DB) *ProviderHintsRepo {
	return &ProviderHintsRepo{db: db}
}

func (r *ProviderHintsRepo) AddHint(ctx context.Context, params repo.AddProviderHintParams) (model.ProviderHint, error) {
	hint := model.ProviderHint{
		HintID:    params.HintID,
		AssetID:   params.AssetID,
		Transport: params.Transport,
		BaseURL:   params.BaseURL,
		Priority:  params.Priority,
		CreatedAt: time.Now().Unix(),
	}

	stmt, err := r.db.PrepareContext(ctx, `
		INSERT INTO provider_hints(hint_id, asset_id, transport, base_url, priority, created_at)
		VALUES(?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return model.ProviderHint{}, fmt.Errorf("prepare add provider hint: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(
		ctx,
		string(hint.HintID),
		string(hint.AssetID),
		hint.Transport,
		hint.BaseURL,
		hint.Priority,
		hint.CreatedAt,
	)
	if err != nil {
		return model.ProviderHint{}, fmt.Errorf("add provider hint: %w", err)
	}

	return hint, nil
}

func (r *ProviderHintsRepo) ListHintsByAsset(ctx context.Context, assetID model.AssetID) ([]model.ProviderHint, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT hint_id, asset_id, transport, base_url, priority, created_at
		FROM provider_hints WHERE asset_id = ? ORDER BY priority ASC, created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare list hints by asset: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, string(assetID))
	if err != nil {
		return nil, fmt.Errorf("list hints by asset: %w", err)
	}
	defer rows.Close()

	hints := make([]model.ProviderHint, 0)
	for rows.Next() {
		var hint model.ProviderHint
		var hintIDRaw string
		var assetIDRaw string
		if err := rows.Scan(
			&hintIDRaw,
			&assetIDRaw,
			&hint.Transport,
			&hint.BaseURL,
			&hint.Priority,
			&hint.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan provider hint row: %w", err)
		}
		hint.HintID = model.ProviderHintID(hintIDRaw)
		hint.AssetID = model.AssetID(assetIDRaw)
		hints = append(hints, hint)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider hint rows: %w", err)
	}

	return hints, nil
}
