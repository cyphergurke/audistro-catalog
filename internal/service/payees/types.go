package payees

import "errors"

var (
	ErrNotFound = errors.New("resource not found")
	ErrConflict = errors.New("payee conflict")
)

type Payee struct {
	PayeeID          string `json:"payee_id"`
	ArtistID         string `json:"artist_id"`
	FAPPublicBaseURL string `json:"fap_public_base_url"`
	FAPPayeeID       string `json:"fap_payee_id"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

type CreatePayeeInput struct {
	ArtistHandle     string
	FAPPublicBaseURL string
	FAPPayeeID       string
}
