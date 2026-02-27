package providers

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cyphergurke/audistro-catalog/internal/model"
	"github.com/cyphergurke/audistro-catalog/internal/store/repo"
	"github.com/cyphergurke/audistro-catalog/internal/validate"
)

type Service struct {
	providersRepo            repo.ProviderRegistryRepository
	assetsRepo               repo.AssetsRepository
	maxAnnounceTTLSeconds    int64
	maxProvidersPerAsset     int64
	insecureTransportAllowed bool
	nowFn                    func() time.Time
}

func NewService(
	providersRepo repo.ProviderRegistryRepository,
	assetsRepo repo.AssetsRepository,
	maxAnnounceTTLSeconds int64,
	maxProvidersPerAsset int64,
	insecureTransportAllowed bool,
) *Service {
	if maxAnnounceTTLSeconds <= 0 {
		maxAnnounceTTLSeconds = 14 * 24 * 60 * 60
	}
	if maxProvidersPerAsset <= 0 {
		maxProvidersPerAsset = 200
	}
	return &Service{
		providersRepo:            providersRepo,
		assetsRepo:               assetsRepo,
		maxAnnounceTTLSeconds:    maxAnnounceTTLSeconds,
		maxProvidersPerAsset:     maxProvidersPerAsset,
		insecureTransportAllowed: insecureTransportAllowed,
		nowFn:                    time.Now,
	}
}

func (s *Service) UpsertProvider(ctx context.Context, input UpsertProviderInput) (Provider, error) {
	normalizedBase, normalizedTransport, _, verr := validate.NormalizeAndValidateBaseURL(input.BaseURL, input.Transport, s.insecureTransportAllowed)
	if verr != nil {
		return Provider{}, fmt.Errorf("validate provider transport/base_url: %w", ErrInvalidInput)
	}
	input.BaseURL = normalizedBase
	input.Transport = normalizedTransport

	if err := validateProviderInput(input); err != nil {
		return Provider{}, fmt.Errorf("validate provider input: %w", err)
	}

	existing, err := s.providersRepo.GetProvider(ctx, input.ProviderID)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return Provider{}, fmt.Errorf("get provider: %w", err)
	}
	if err == nil && !strings.EqualFold(existing.PublicKey, input.PublicKey) {
		return Provider{}, fmt.Errorf("provider public key mismatch: %w", ErrConflict)
	}

	stored, err := s.providersRepo.UpsertProvider(ctx, repo.UpsertProviderParams{
		ProviderID: input.ProviderID,
		PublicKey:  strings.ToLower(input.PublicKey),
		Transport:  input.Transport,
		BaseURL:    input.BaseURL,
		Region:     input.Region,
		Status:     "active",
	})
	if err != nil {
		return Provider{}, fmt.Errorf("upsert provider: %w", err)
	}

	return toProvider(stored), nil
}

func (s *Service) Announce(ctx context.Context, input AnnounceInput) error {
	expiresAt, normalizedBase, normalizedTransport, err := s.validateAnnounceInput(input)
	if err != nil {
		return err
	}

	provider, err := s.providersRepo.GetProvider(ctx, input.ProviderID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return fmt.Errorf("provider not found: %w", ErrNotFound)
		}
		return fmt.Errorf("get provider: %w", err)
	}

	if _, err := s.assetsRepo.GetAsset(ctx, model.AssetID(input.AssetID)); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return fmt.Errorf("asset not found: %w", ErrNotFound)
		}
		return fmt.Errorf("get asset: %w", err)
	}

	now := s.nowFn().Unix()
	existingAnnouncement, getErr := s.providersRepo.GetProviderAssetAnnouncement(ctx, input.ProviderID, model.AssetID(input.AssetID))
	exists := getErr == nil
	if getErr != nil && !errors.Is(getErr, repo.ErrNotFound) {
		return fmt.Errorf("get provider announcement: %w", getErr)
	}

	if !exists {
		count, countErr := s.providersRepo.CountActiveProvidersByAsset(ctx, model.AssetID(input.AssetID), now)
		if countErr != nil {
			return fmt.Errorf("count active providers by asset: %w", countErr)
		}
		if count >= s.maxProvidersPerAsset {
			return fmt.Errorf("asset providers capacity exceeded: %w", ErrCapacityExceeded)
		}
	} else if existingAnnouncement.ExpiresAt <= now {
		count, countErr := s.providersRepo.CountActiveProvidersByAsset(ctx, model.AssetID(input.AssetID), now)
		if countErr != nil {
			return fmt.Errorf("count active providers by asset: %w", countErr)
		}
		if count >= s.maxProvidersPerAsset {
			return fmt.Errorf("asset providers capacity exceeded: %w", ErrCapacityExceeded)
		}
	}

	message := canonicalAnnouncementMessage(input.ProviderID, input.AssetID, normalizedTransport, normalizedBase, expiresAt, strings.ToLower(input.Nonce))
	if err := verifyAnnouncementSignature(provider.PublicKey, input.Signature, message); err != nil {
		return fmt.Errorf("verify announcement signature: %w", ErrInvalidInput)
	}

	_, err = s.providersRepo.UpsertProviderAssetAnnouncement(ctx, repo.UpsertProviderAssetAnnouncementParams{
		ProviderID: input.ProviderID,
		AssetID:    model.AssetID(input.AssetID),
		Transport:  normalizedTransport,
		BaseURL:    normalizedBase,
		Priority:   input.Priority,
		ExpiresAt:  expiresAt,
		LastSeenAt: now,
		Nonce:      strings.ToLower(input.Nonce),
	})
	if err != nil {
		return fmt.Errorf("upsert provider announcement: %w", err)
	}

	return nil
}

func (s *Service) ListAssetProviders(ctx context.Context, assetID string, limit int) ([]AssetProvider, error) {
	if strings.TrimSpace(assetID) == "" {
		return nil, fmt.Errorf("asset id is required: %w", ErrInvalidInput)
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive: %w", ErrInvalidInput)
	}

	if _, err := s.assetsRepo.GetAsset(ctx, model.AssetID(assetID)); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, fmt.Errorf("asset not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("get asset: %w", err)
	}

	now := s.nowFn().Unix()
	hints, err := s.providersRepo.ListActiveProvidersByAsset(ctx, model.AssetID(assetID), now, limit)
	if err != nil {
		return nil, fmt.Errorf("list providers by asset: %w", err)
	}

	result := make([]AssetProvider, 0, len(hints))
	for _, hint := range hints {
		result = append(result, AssetProvider{
			ProviderID: hint.ProviderID,
			Transport:  hint.Transport,
			BaseURL:    hint.BaseURL,
			Priority:   hint.Priority,
			ExpiresAt:  hint.ExpiresAt,
			LastSeenAt: hint.LastSeenAt,
			UpdatedAt:  hint.UpdatedAt,
			Region:     hint.Region,
		})
	}
	return result, nil
}

func (s *Service) validateAnnounceInput(input AnnounceInput) (int64, string, string, error) {
	if strings.TrimSpace(input.ProviderID) == "" {
		return 0, "", "", fmt.Errorf("provider_id is required: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(input.AssetID) == "" {
		return 0, "", "", fmt.Errorf("asset_id is required: %w", ErrInvalidInput)
	}
	normalizedBase, normalizedTransport, _, verr := validate.NormalizeAndValidateBaseURL(input.BaseURL, input.Transport, s.insecureTransportAllowed)
	if verr != nil {
		return 0, "", "", fmt.Errorf("invalid announce base_url/transport: %w", ErrInvalidInput)
	}
	if !strings.Contains(normalizedBase, "/assets/"+input.AssetID) {
		return 0, "", "", fmt.Errorf("base_url must include /assets/{assetId}: %w", ErrInvalidInput)
	}
	if input.Priority < 0 || input.Priority > 100 {
		return 0, "", "", fmt.Errorf("priority must be in range 0..100: %w", ErrInvalidInput)
	}
	if verr := validateNonce(input.Nonce); verr != nil {
		return 0, "", "", verr
	}
	if strings.TrimSpace(input.Signature) == "" {
		return 0, "", "", fmt.Errorf("signature is required: %w", ErrInvalidInput)
	}

	now := s.nowFn().Unix()
	maxExpires := now + s.maxAnnounceTTLSeconds
	expiresAt := input.ExpiresAt
	if expiresAt == 0 && input.ExpiresInSeconds > 0 {
		expiresAt = now + input.ExpiresInSeconds
	}
	if expiresAt <= now {
		return 0, "", "", fmt.Errorf("expires_at must be in the future: %w", ErrInvalidInput)
	}
	if expiresAt > maxExpires {
		return 0, "", "", fmt.Errorf("expires_at exceeds max ttl: %w", ErrInvalidInput)
	}

	return expiresAt, normalizedBase, normalizedTransport, nil
}

func validateProviderInput(input UpsertProviderInput) error {
	if strings.TrimSpace(input.ProviderID) == "" {
		return fmt.Errorf("provider_id is required: %w", ErrInvalidInput)
	}
	if err := validatePublicKey(input.PublicKey); err != nil {
		return err
	}
	if len(input.Region) > 64 {
		return fmt.Errorf("region too long: %w", ErrInvalidInput)
	}
	return nil
}

func validatePublicKey(value string) error {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("public_key must be hex: %w", ErrInvalidInput)
	}
	if len(decoded) != 33 {
		return fmt.Errorf("public_key must be 33 bytes compressed: %w", ErrInvalidInput)
	}
	return nil
}

func validateNonce(nonce string) error {
	trimmed := strings.TrimSpace(nonce)
	if len(trimmed) < 16 || len(trimmed) > 128 {
		return fmt.Errorf("nonce length must be 16..128 hex chars: %w", ErrInvalidInput)
	}
	if len(trimmed)%2 != 0 {
		return fmt.Errorf("nonce must have even-length hex: %w", ErrInvalidInput)
	}
	if _, err := hex.DecodeString(trimmed); err != nil {
		return fmt.Errorf("nonce must be hex: %w", ErrInvalidInput)
	}
	return nil
}

func toProvider(value model.Provider) Provider {
	return Provider{
		ProviderID: value.ProviderID,
		PublicKey:  value.PublicKey,
		Transport:  value.Transport,
		BaseURL:    value.BaseURL,
		Region:     value.Region,
		Status:     value.Status,
		CreatedAt:  value.CreatedAt,
		UpdatedAt:  value.UpdatedAt,
	}
}
