package assets

import "errors"

var (
	ErrNotFound      = errors.New("resource not found")
	ErrConflict      = errors.New("asset conflict")
	ErrPayeeMismatch = errors.New("payee does not belong to artist")
)

type Moderation struct {
	State string `json:"state"`
}

type Purchase struct {
	ResourceID   string `json:"resource_id"`
	ChallengeURL string `json:"challenge_url"`
	TokenURL     string `json:"token_url"`
	PayeeID      string `json:"payee_id"`
	FAPPayeeID   string `json:"fap_payee_id"`
	PriceMSat    int64  `json:"price_msat"`
}

type ProviderHint struct {
	HintID    string `json:"hint_id"`
	AssetID   string `json:"asset_id"`
	Transport string `json:"transport"`
	BaseURL   string `json:"base_url"`
	Priority  int64  `json:"priority"`
	CreatedAt int64  `json:"created_at"`
}

type ArtistSummary struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	PubKeyHex   string `json:"pubkey_hex"`
}

type Asset struct {
	AssetID            string     `json:"asset_id"`
	ArtistID           string     `json:"artist_id"`
	PayeeID            string     `json:"payee_id"`
	Title              string     `json:"title"`
	DurationMS         int64      `json:"duration_ms"`
	ContentID          string     `json:"content_id"`
	HLSMasterURL       string     `json:"hls_master_url"`
	PreviewHLSURL      string     `json:"preview_hls_url"`
	PriceMSat          int64      `json:"price_msat"`
	CreatedAt          int64      `json:"created_at"`
	UpdatedAt          int64      `json:"updated_at"`
	Purchase           Purchase   `json:"purchase"`
	Moderation         Moderation `json:"moderation"`
	ProviderHintsCount int        `json:"provider_hints_count"`
}

type AssetWithArtist struct {
	Asset  Asset         `json:"asset"`
	Artist ArtistSummary `json:"artist"`
}

type CreateProviderHintInput struct {
	Transport string
	BaseURL   string
	Priority  int64
}

type CreateAssetInput struct {
	AssetID       string
	ArtistHandle  string
	PayeeID       string
	Title         string
	DurationMS    int64
	ContentID     string
	HLSMasterURL  string
	PreviewHLSURL string
	PriceMSat     int64
	ProviderHints []CreateProviderHintInput
}
