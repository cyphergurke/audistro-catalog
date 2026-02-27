package repo

import (
	"context"

	"github.com/cyphergurke/audistro-catalog/internal/model"
)

type CreateArtistParams struct {
	ArtistID    model.ArtistID
	PubKeyHex   string
	Handle      string
	DisplayName string
	Bio         string
	AvatarURL   string
}

type ArtistsRepository interface {
	CreateArtist(ctx context.Context, params CreateArtistParams) (model.Artist, error)
	GetArtistByID(ctx context.Context, artistID model.ArtistID) (model.Artist, error)
	GetArtistByHandle(ctx context.Context, handle string) (model.Artist, error)
	GetArtistByPubKey(ctx context.Context, pubKeyHex string) (model.Artist, error)
	ListArtists(ctx context.Context) ([]model.Artist, error)
}
