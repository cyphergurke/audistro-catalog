package repo

import (
	"context"

	"audistro-catalog/internal/model"
)

type UpsertVerificationStateParams struct {
	PubKeyHex  string
	Badge      string
	Score      int64
	ComputedAt int64
	InputsHash string
}

type VerificationRepository interface {
	GetByPubKeyHex(ctx context.Context, pubKeyHex string) (model.VerificationState, error)
	UpsertState(ctx context.Context, params UpsertVerificationStateParams) (model.VerificationState, error)
}
