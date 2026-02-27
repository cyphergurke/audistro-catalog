package model

// ProviderHint gives optional transport endpoints for playback.
type ProviderHint struct {
	HintID    ProviderHintID
	AssetID   AssetID
	Transport string
	BaseURL   string
	Priority  int64
	CreatedAt int64
}
