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

var _ repo.ModerationRepository = (*ModerationRepo)(nil)

type ModerationRepo struct {
	db *sql.DB
}

func NewModerationRepo(db *sql.DB) *ModerationRepo {
	return &ModerationRepo{db: db}
}

func (r *ModerationRepo) GetState(ctx context.Context, targetType string, targetID string) (model.ModerationState, bool, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT target_type, target_id, state, reason_code, updated_at
		FROM moderation_state WHERE target_type = ? AND target_id = ?
	`)
	if err != nil {
		return model.ModerationState{}, false, fmt.Errorf("prepare get moderation state: %w", err)
	}
	defer stmt.Close()

	state, err := scanModerationState(stmt.QueryRowContext(ctx, targetType, targetID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ModerationState{}, false, nil
		}
		return model.ModerationState{}, false, fmt.Errorf("get moderation state: %w", err)
	}
	return state, true, nil
}

func (r *ModerationRepo) UpsertState(ctx context.Context, params repo.UpsertModerationStateParams) (model.ModerationState, error) {
	state := model.ModerationState{
		TargetType: params.TargetType,
		TargetID:   params.TargetID,
		State:      params.State,
		ReasonCode: params.ReasonCode,
		UpdatedAt:  time.Now().Unix(),
	}

	stmt, err := r.db.PrepareContext(ctx, `
		INSERT INTO moderation_state(target_type, target_id, state, reason_code, updated_at)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(target_type, target_id) DO UPDATE SET
			state = excluded.state,
			reason_code = excluded.reason_code,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return model.ModerationState{}, fmt.Errorf("prepare upsert moderation state: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(
		ctx,
		state.TargetType,
		state.TargetID,
		state.State,
		state.ReasonCode,
		state.UpdatedAt,
	)
	if err != nil {
		return model.ModerationState{}, fmt.Errorf("upsert moderation state: %w", err)
	}

	return state, nil
}

func scanModerationState(row *sql.Row) (model.ModerationState, error) {
	var state model.ModerationState
	if err := row.Scan(
		&state.TargetType,
		&state.TargetID,
		&state.State,
		&state.ReasonCode,
		&state.UpdatedAt,
	); err != nil {
		return model.ModerationState{}, err
	}
	return state, nil
}
