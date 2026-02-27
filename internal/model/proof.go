package model

type ProofType string

const (
	ProofTypeDomainTXT ProofType = "domain_txt"
	ProofTypeWellKnown ProofType = "well_known"
)

type ProofStatus string

const (
	ProofStatusPending  ProofStatus = "pending"
	ProofStatusVerified ProofStatus = "verified"
	ProofStatusFailed   ProofStatus = "failed"
	ProofStatusExpired  ProofStatus = "expired"
)

// ArtistProof binds an artist pubkey to an externally verifiable proof.
type ArtistProof struct {
	ProofID         string
	ArtistPubKeyHex string
	ProofType       ProofType
	ProofValue      string
	Status          ProofStatus
	CheckedAt       int64
	Details         string
	CreatedAt       int64
	UpdatedAt       int64
}
