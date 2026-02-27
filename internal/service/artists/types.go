package artists

import "errors"

var (
	ErrNotFound = errors.New("artist not found")
	ErrConflict = errors.New("artist conflict")
)

type Verification struct {
	Badge string `json:"badge"`
	Score int    `json:"score"`
}

type Moderation struct {
	State string `json:"state"`
}

type Artist struct {
	ArtistID     string       `json:"artist_id"`
	PubKeyHex    string       `json:"pubkey_hex"`
	Handle       string       `json:"handle"`
	DisplayName  string       `json:"display_name"`
	Bio          string       `json:"bio"`
	AvatarURL    string       `json:"avatar_url"`
	CreatedAt    int64        `json:"created_at"`
	UpdatedAt    int64        `json:"updated_at"`
	Verification Verification `json:"verification"`
	Moderation   Moderation   `json:"moderation"`
}

type CreateArtistInput struct {
	PubKeyHex   string
	Handle      string
	DisplayName string
	Bio         string
	AvatarURL   string
}
