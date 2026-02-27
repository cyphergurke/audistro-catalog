package repo

import (
	"context"

	"github.com/cyphergurke/audistro-catalog/internal/model"
)

type ListProofsByStatusParams struct {
	Statuses []model.ProofStatus
	Limit    int
}

type UpdateProofStatusParams struct {
	ProofID   string
	Status    model.ProofStatus
	CheckedAt int64
	Details   string
	UpdatedAt int64
}

type ProofsRepository interface {
	ListByStatus(ctx context.Context, params ListProofsByStatusParams) ([]model.ArtistProof, error)
	UpdateStatus(ctx context.Context, params UpdateProofStatusParams) error
	ListByArtistPubKey(ctx context.Context, pubKeyHex string) ([]model.ArtistProof, error)
}
