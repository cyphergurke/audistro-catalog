package model

// VerificationState stores verification metadata by pubkey.
type VerificationState struct {
	PubKeyHex  string
	Badge      string
	Score      int64
	UpdatedAt  int64
	ComputedAt int64
	InputsHash string
}
