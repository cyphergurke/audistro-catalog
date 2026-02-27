package repo

import (
	"context"

	"audistro-catalog/internal/model"
)

type UpsertModerationStateParams struct {
	TargetType string
	TargetID   string
	State      string
	ReasonCode string
}

type ModerationRepository interface {
	GetState(ctx context.Context, targetType string, targetID string) (model.ModerationState, bool, error)
	UpsertState(ctx context.Context, params UpsertModerationStateParams) (model.ModerationState, error)
}
