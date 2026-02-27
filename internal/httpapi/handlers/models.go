package handlers

type PlaybackResponse struct {
	APIVersion    string                 `json:"api_version"`
	SchemaVersion int                    `json:"schema_version"`
	Now           int64                  `json:"now"`
	Asset         AssetPlayback          `json:"asset"`
	Providers     []AssetProviderPayload `json:"providers"`
}

type AssetPlayback struct {
	AssetID    string       `json:"asset_id"`
	Title      string       `json:"title,omitempty"`
	DurationMS int64        `json:"duration_ms,omitempty"`
	HLS        PlaybackHLS  `json:"hls"`
	Pay        *PlaybackPay `json:"pay,omitempty"`
}

type PlaybackHLS struct {
	MasterPath     string `json:"master_path,omitempty"`
	Encryption     string `json:"encryption,omitempty"`
	KeyURITemplate string `json:"key_uri_template,omitempty"`
}

type PlaybackPay struct {
	ResourceID   string `json:"resource_id,omitempty"`
	ChallengeURL string `json:"challenge_url,omitempty"`
	TokenURL     string `json:"token_url,omitempty"`
	FAPURL       string `json:"fap_url,omitempty"`
	PayeeID      string `json:"payee_id,omitempty"`
	FAPPayeeID   string `json:"fap_payee_id,omitempty"`
	PriceMSat    int64  `json:"price_msat"`
}
