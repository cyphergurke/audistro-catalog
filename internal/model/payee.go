package model

// Payee maps an artist to an external FAP payee endpoint.
type Payee struct {
	PayeeID          PayeeID
	ArtistID         ArtistID
	FAPPublicBaseURL string
	FAPPayeeID       string
	CreatedAt        int64
	UpdatedAt        int64
}
