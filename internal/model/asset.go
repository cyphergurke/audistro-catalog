package model

// Asset stores catalog metadata for playable content.
type Asset struct {
	AssetID       AssetID
	ArtistID      ArtistID
	PayeeID       PayeeID
	Title         string
	DurationMS    int64
	ContentID     string
	HLSMasterURL  string
	PreviewHLSURL string
	PriceMSat     int64
	CreatedAt     int64
	UpdatedAt     int64
}
