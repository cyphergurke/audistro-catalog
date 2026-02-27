package repo

import (
	"context"

	"audistro-catalog/internal/model"
)

type CreatePayeeParams struct {
	PayeeID          model.PayeeID
	ArtistID         model.ArtistID
	FAPPublicBaseURL string
	FAPPayeeID       string
}

type PayeesRepository interface {
	CreatePayee(ctx context.Context, params CreatePayeeParams) (model.Payee, error)
	GetPayee(ctx context.Context, payeeID model.PayeeID) (model.Payee, error)
	ListPayeesByArtist(ctx context.Context, artistID model.ArtistID) ([]model.Payee, error)
}
