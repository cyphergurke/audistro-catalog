package model

// Artist stores identity metadata for an artist.
type Artist struct {
	ArtistID    ArtistID
	PubKeyHex   string
	Handle      string
	DisplayName string
	Bio         string
	AvatarURL   string
	CreatedAt   int64
	UpdatedAt   int64
}
