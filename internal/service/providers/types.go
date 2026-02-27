package providers

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrConflict         = errors.New("conflict")
	ErrInvalidInput     = errors.New("invalid input")
	ErrCapacityExceeded = errors.New("capacity exceeded")
)

type Provider struct {
	ProviderID string `json:"provider_id"`
	PublicKey  string `json:"public_key"`
	Transport  string `json:"transport"`
	BaseURL    string `json:"base_url"`
	Region     string `json:"region,omitempty"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

type AssetProvider struct {
	ProviderID string `json:"provider_id"`
	Transport  string `json:"transport"`
	BaseURL    string `json:"base_url"`
	Priority   int64  `json:"priority"`
	ExpiresAt  int64  `json:"expires_at"`
	LastSeenAt int64  `json:"last_seen_at"`
	UpdatedAt  int64  `json:"updated_at"`
	Region     string `json:"region,omitempty"`
}

type UpsertProviderInput struct {
	ProviderID string
	PublicKey  string
	Transport  string
	BaseURL    string
	Region     string
}

type AnnounceInput struct {
	ProviderID       string
	AssetID          string
	Transport        string
	BaseURL          string
	Priority         int64
	ExpiresInSeconds int64
	ExpiresAt        int64
	Nonce            string
	Signature        string
}
