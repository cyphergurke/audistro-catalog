package proofcheck

import (
	"context"

	"audistro-catalog/internal/store/repo"
)

type DNSResolver interface {
	LookupTXT(ctx context.Context, domain string) ([]string, error)
}

type HTTPFetcher interface {
	Get(ctx context.Context, url string) (status int, body []byte, err error)
}

type Dependencies struct {
	ProofsRepo       repo.ProofsRepository
	ArtistsRepo      repo.ArtistsRepository
	ModerationRepo   repo.ModerationRepository
	VerificationRepo repo.VerificationRepository
	DNSResolver      DNSResolver
	HTTPFetcher      HTTPFetcher
	Limit            int
}

type Result struct {
	ProcessedProofs int
	AffectedArtists int
}
