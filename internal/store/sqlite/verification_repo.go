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

var _ repo.VerificationRepository = (*VerificationRepo)(nil)

type VerificationRepo struct {
	db *sql.DB
}

func NewVerificationRepo(db *sql.DB) *VerificationRepo {
	return &VerificationRepo{db: db}
}

func (r *VerificationRepo) GetByPubKeyHex(ctx context.Context, pubKeyHex string) (model.VerificationState, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT pubkey_hex, badge, score, updated_at, computed_at, inputs_hash
		FROM verification_state
		WHERE pubkey_hex = ?
	`)
	if err != nil {
		return model.VerificationState{}, fmt.Errorf("prepare get verification state: %w", err)
	}
	defer stmt.Close()

	var state model.VerificationState
	if err := stmt.QueryRowContext(ctx, pubKeyHex).Scan(
		&state.PubKeyHex,
		&state.Badge,
		&state.Score,
		&state.UpdatedAt,
		&state.ComputedAt,
		&state.InputsHash,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.VerificationState{}, fmt.Errorf("get verification state: %w", repo.ErrNotFound)
		}
		return model.VerificationState{}, fmt.Errorf("get verification state: %w", err)
	}

	return state, nil
}

func (r *VerificationRepo) UpsertState(ctx context.Context, params repo.UpsertVerificationStateParams) (model.VerificationState, error) {
	computedAt := params.ComputedAt
	if computedAt == 0 {
		computedAt = time.Now().Unix()
	}
	state := model.VerificationState{
		PubKeyHex:  params.PubKeyHex,
		Badge:      params.Badge,
		Score:      params.Score,
		UpdatedAt:  computedAt,
		ComputedAt: computedAt,
		InputsHash: params.InputsHash,
	}

	stmt, err := r.db.PrepareContext(ctx, `
		INSERT INTO verification_state(pubkey_hex, badge, score, updated_at, computed_at, inputs_hash)
		VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(pubkey_hex) DO UPDATE SET
			badge = excluded.badge,
			score = excluded.score,
			updated_at = excluded.updated_at,
			computed_at = excluded.computed_at,
			inputs_hash = excluded.inputs_hash
	`)
	if err != nil {
		return model.VerificationState{}, fmt.Errorf("prepare upsert verification state: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(
		ctx,
		state.PubKeyHex,
		state.Badge,
		state.Score,
		state.UpdatedAt,
		state.ComputedAt,
		state.InputsHash,
	)
	if err != nil {
		return model.VerificationState{}, fmt.Errorf("upsert verification state: %w", err)
	}

	return state, nil
}
