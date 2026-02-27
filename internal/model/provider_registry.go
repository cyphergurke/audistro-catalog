package model

// Provider contains provider registry metadata.
type Provider struct {
	ProviderID string
	PublicKey  string
	Transport  string
	BaseURL    string
	Region     string
	Status     string
	CreatedAt  int64
	UpdatedAt  int64
}

// ProviderAssetAnnouncement stores untrusted provider announcements.
type ProviderAssetAnnouncement struct {
	ProviderID string
	AssetID    AssetID
	Transport  string
	BaseURL    string
	Priority   int64
	ExpiresAt  int64
	LastSeenAt int64
	Nonce      string
	CreatedAt  int64
	UpdatedAt  int64
}

// AssetProviderHint is a provider view returned for an asset.
type AssetProviderHint struct {
	ProviderID string
	Transport  string
	BaseURL    string
	Priority   int64
	ExpiresAt  int64
	LastSeenAt int64
	UpdatedAt  int64
	Region     string
}
