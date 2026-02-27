package proofcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"audistro-catalog/internal/model"
	verifsvc "audistro-catalog/internal/service/verification"
	"audistro-catalog/internal/store/repo"
)

type Worker struct {
	proofsRepo       repo.ProofsRepository
	artistsRepo      repo.ArtistsRepository
	moderationRepo   repo.ModerationRepository
	verificationRepo repo.VerificationRepository
	dnsResolver      DNSResolver
	httpFetcher      HTTPFetcher
	limit            int
	nowFn            func() time.Time
}

func NewWorker(deps Dependencies) (*Worker, error) {
	if deps.ProofsRepo == nil {
		return nil, errors.New("proofs repo is required")
	}
	if deps.ArtistsRepo == nil {
		return nil, errors.New("artists repo is required")
	}
	if deps.ModerationRepo == nil {
		return nil, errors.New("moderation repo is required")
	}
	if deps.VerificationRepo == nil {
		return nil, errors.New("verification repo is required")
	}
	if deps.DNSResolver == nil {
		return nil, errors.New("dns resolver is required")
	}
	if deps.HTTPFetcher == nil {
		return nil, errors.New("http fetcher is required")
	}

	limit := deps.Limit
	if limit <= 0 {
		limit = 200
	}

	return &Worker{
		proofsRepo:       deps.ProofsRepo,
		artistsRepo:      deps.ArtistsRepo,
		moderationRepo:   deps.ModerationRepo,
		verificationRepo: deps.VerificationRepo,
		dnsResolver:      deps.DNSResolver,
		httpFetcher:      deps.HTTPFetcher,
		limit:            limit,
		nowFn:            time.Now,
	}, nil
}

func (w *Worker) RunOnce(ctx context.Context) (Result, error) {
	proofs, err := w.proofsRepo.ListByStatus(ctx, repo.ListProofsByStatusParams{
		Statuses: []model.ProofStatus{model.ProofStatusPending, model.ProofStatusExpired},
		Limit:    w.limit,
	})
	if err != nil {
		return Result{}, fmt.Errorf("list proofs by status: %w", err)
	}

	nowUnix := w.nowFn().Unix()
	affected := make(map[string]struct{})
	for _, proof := range proofs {
		status, details := w.verifyProof(ctx, proof)
		if err := w.proofsRepo.UpdateStatus(ctx, repo.UpdateProofStatusParams{
			ProofID:   proof.ProofID,
			Status:    status,
			CheckedAt: nowUnix,
			Details:   details,
			UpdatedAt: nowUnix,
		}); err != nil {
			return Result{}, fmt.Errorf("update proof status for %s: %w", proof.ProofID, err)
		}
		affected[proof.ArtistPubKeyHex] = struct{}{}
	}

	for pubKeyHex := range affected {
		if err := w.recomputeVerificationState(ctx, pubKeyHex, nowUnix); err != nil {
			return Result{}, fmt.Errorf("recompute verification for pubkey %s: %w", pubKeyHex, err)
		}
	}

	return Result{ProcessedProofs: len(proofs), AffectedArtists: len(affected)}, nil
}

func (w *Worker) recomputeVerificationState(ctx context.Context, pubKeyHex string, nowUnix int64) error {
	proofs, err := w.proofsRepo.ListByArtistPubKey(ctx, pubKeyHex)
	if err != nil {
		return fmt.Errorf("list proofs by artist pubkey: %w", err)
	}

	var moderation *model.ModerationState
	artist, err := w.artistsRepo.GetArtistByPubKey(ctx, pubKeyHex)
	if err != nil {
		if !errors.Is(err, repo.ErrNotFound) {
			return fmt.Errorf("get artist by pubkey: %w", err)
		}
	} else {
		state, exists, stateErr := w.moderationRepo.GetState(ctx, "artist", string(artist.ArtistID))
		if stateErr != nil {
			return fmt.Errorf("get moderation state: %w", stateErr)
		}
		if exists {
			moderation = &state
		}
	}

	scored := verifsvc.ComputeState(proofs, moderation)
	_, err = w.verificationRepo.UpsertState(ctx, repo.UpsertVerificationStateParams{
		PubKeyHex:  pubKeyHex,
		Badge:      scored.Badge,
		Score:      scored.Score,
		ComputedAt: nowUnix,
		InputsHash: scored.InputsHash,
	})
	if err != nil {
		return fmt.Errorf("upsert verification state: %w", err)
	}

	return nil
}

func (w *Worker) verifyProof(ctx context.Context, proof model.ArtistProof) (model.ProofStatus, string) {
	switch proof.ProofType {
	case model.ProofTypeDomainTXT:
		return w.verifyDomainTXT(ctx, proof)
	case model.ProofTypeWellKnown:
		return w.verifyWellKnown(ctx, proof)
	default:
		return model.ProofStatusFailed, "unsupported_proof_type"
	}
}

func (w *Worker) verifyDomainTXT(ctx context.Context, proof model.ArtistProof) (model.ProofStatus, string) {
	domain := strings.TrimSpace(proof.ProofValue)
	if domain == "" {
		return model.ProofStatusFailed, "invalid_domain"
	}

	txtRecords, err := w.dnsResolver.LookupTXT(ctx, domain)
	if err != nil {
		return model.ProofStatusFailed, "dns_lookup_error"
	}

	expected := "fap-verification=" + proof.ArtistPubKeyHex
	for _, record := range txtRecords {
		if record == expected {
			return model.ProofStatusVerified, "matched_txt"
		}
	}
	return model.ProofStatusFailed, "txt_mismatch"
}

func (w *Worker) verifyWellKnown(ctx context.Context, proof model.ArtistProof) (model.ProofStatus, string) {
	endpoint, err := buildWellKnownURL(proof.ProofValue)
	if err != nil {
		return model.ProofStatusFailed, "invalid_domain"
	}

	statusCode, body, fetchErr := w.httpFetcher.Get(ctx, endpoint)
	if fetchErr != nil {
		return model.ProofStatusFailed, "fetch_error"
	}
	if statusCode != 200 {
		return model.ProofStatusFailed, fmt.Sprintf("http_status_%d", statusCode)
	}

	var payload struct {
		PubKeyHex string `json:"pubkey_hex"`
		UpdatedAt int64  `json:"updated_at"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return model.ProofStatusFailed, "invalid_json"
	}
	if payload.PubKeyHex == proof.ArtistPubKeyHex {
		return model.ProofStatusVerified, "matched_well_known"
	}
	return model.ProofStatusFailed, "well_known_mismatch"
}

func buildWellKnownURL(proofValue string) (string, error) {
	raw := strings.TrimSpace(proofValue)
	if raw == "" {
		return "", errors.New("empty proof value")
	}

	host := raw
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", err
		}
		host = parsed.Host
	}
	if host == "" {
		return "", errors.New("empty host")
	}
	if strings.Contains(host, "/") {
		return "", errors.New("host contains path")
	}

	return "https://" + host + "/.well-known/fap.json", nil
}
