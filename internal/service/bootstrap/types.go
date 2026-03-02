package bootstrap

import "errors"

var (
	ErrNotFound = errors.New("resource not found")
	ErrConflict = errors.New("bootstrap conflict")
)

type BootstrapArtistInput struct {
	ArtistID    string
	Handle      string
	DisplayName string
	PubKeyHex   string
	Payee       BootstrapPayeeInput
}

type BootstrapPayeeInput struct {
	PayeeID          string
	FAPPublicBaseURL string
	FAPPayeeID       string
}

type BootstrapArtistResult struct {
	ArtistID   string `json:"artist_id"`
	PayeeID    string `json:"payee_id"`
	Handle     string `json:"handle"`
	FAPPayeeID string `json:"fap_payee_id"`
}
