package assets

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sort"
	"strings"

	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/cyphergurke/audistro-catalog/internal/model"
	"github.com/cyphergurke/audistro-catalog/internal/store/repo"
)

type Service struct {
	artistsRepo       repo.ArtistsRepository
	payeesRepo        repo.PayeesRepository
	assetsRepo        repo.AssetsRepository
	providerHintsRepo repo.ProviderHintsRepository
	moderationRepo    repo.ModerationRepository
}

func NewService(
	artistsRepo repo.ArtistsRepository,
	payeesRepo repo.PayeesRepository,
	assetsRepo repo.AssetsRepository,
	providerHintsRepo repo.ProviderHintsRepository,
	moderationRepo repo.ModerationRepository,
) *Service {
	return &Service{
		artistsRepo:       artistsRepo,
		payeesRepo:        payeesRepo,
		assetsRepo:        assetsRepo,
		providerHintsRepo: providerHintsRepo,
		moderationRepo:    moderationRepo,
	}
}

func (s *Service) CreateAsset(ctx context.Context, input CreateAssetInput) (Asset, []ProviderHint, error) {
	artist, err := s.artistsRepo.GetArtistByHandle(ctx, input.ArtistHandle)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return Asset{}, nil, fmt.Errorf("get artist for create asset: %w", ErrNotFound)
		}
		return Asset{}, nil, fmt.Errorf("get artist for create asset: %w", err)
	}

	payee, err := s.payeesRepo.GetPayee(ctx, model.PayeeID(input.PayeeID))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return Asset{}, nil, fmt.Errorf("get payee for create asset: %w", ErrNotFound)
		}
		return Asset{}, nil, fmt.Errorf("get payee for create asset: %w", err)
	}
	if payee.ArtistID != artist.ArtistID {
		return Asset{}, nil, fmt.Errorf("payee artist mismatch: %w", ErrPayeeMismatch)
	}

	assetID := input.AssetID
	if assetID == "" {
		assetID = generateUUID()
	}

	created, err := s.assetsRepo.CreateAsset(ctx, repo.CreateAssetParams{
		AssetID:       model.AssetID(assetID),
		ArtistID:      artist.ArtistID,
		PayeeID:       payee.PayeeID,
		Title:         input.Title,
		DurationMS:    input.DurationMS,
		ContentID:     input.ContentID,
		HLSMasterURL:  input.HLSMasterURL,
		PreviewHLSURL: input.PreviewHLSURL,
		PriceMSat:     input.PriceMSat,
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return Asset{}, nil, fmt.Errorf("create asset conflict: %w", ErrConflict)
		}
		return Asset{}, nil, fmt.Errorf("create asset: %w", err)
	}

	hints := make([]ProviderHint, 0, len(input.ProviderHints))
	for _, hintInput := range input.ProviderHints {
		hint, addErr := s.AddProviderHint(ctx, assetID, hintInput)
		if addErr != nil {
			if errors.Is(addErr, ErrConflict) {
				return Asset{}, nil, fmt.Errorf("add provider hint conflict: %w", ErrConflict)
			}
			return Asset{}, nil, fmt.Errorf("add provider hint: %w", addErr)
		}
		hints = append(hints, hint)
	}

	asset := toAsset(created, payee, len(hints), moderationStateAllow())
	return asset, hints, nil
}

func (s *Service) GetAsset(ctx context.Context, assetID string) (AssetWithArtist, error) {
	assetModel, err := s.assetsRepo.GetAsset(ctx, model.AssetID(assetID))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return AssetWithArtist{}, fmt.Errorf("get asset: %w", ErrNotFound)
		}
		return AssetWithArtist{}, fmt.Errorf("get asset: %w", err)
	}

	artist, err := s.artistsRepo.GetArtistByID(ctx, assetModel.ArtistID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return AssetWithArtist{}, fmt.Errorf("get artist for asset: %w", ErrNotFound)
		}
		return AssetWithArtist{}, fmt.Errorf("get artist for asset: %w", err)
	}

	payee, err := s.payeesRepo.GetPayee(ctx, assetModel.PayeeID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return AssetWithArtist{}, fmt.Errorf("get payee for asset: %w", ErrNotFound)
		}
		return AssetWithArtist{}, fmt.Errorf("get payee for asset: %w", err)
	}

	hints, err := s.providerHintsRepo.ListHintsByAsset(ctx, assetModel.AssetID)
	if err != nil {
		return AssetWithArtist{}, fmt.Errorf("list provider hints for asset: %w", err)
	}

	moderationState, err := s.getModerationState(ctx, "asset", string(assetModel.AssetID))
	if err != nil {
		return AssetWithArtist{}, fmt.Errorf("get moderation for asset: %w", err)
	}

	asset := toAsset(assetModel, payee, len(hints), moderationState)
	return AssetWithArtist{
		Asset: asset,
		Artist: ArtistSummary{
			Handle:      artist.Handle,
			DisplayName: artist.DisplayName,
			PubKeyHex:   artist.PubKeyHex,
		},
	}, nil
}

func (s *Service) ListAssetsByArtistHandle(ctx context.Context, handle string) ([]Asset, error) {
	artist, err := s.artistsRepo.GetArtistByHandle(ctx, handle)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, fmt.Errorf("get artist for list assets: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("get artist for list assets: %w", err)
	}

	assetsList, err := s.assetsRepo.ListAssetsByArtist(ctx, artist.ArtistID)
	if err != nil {
		return nil, fmt.Errorf("list assets by artist: %w", err)
	}

	result := make([]Asset, 0, len(assetsList))
	for _, assetModel := range assetsList {
		moderationState, mErr := s.getModerationState(ctx, "asset", string(assetModel.AssetID))
		if mErr != nil {
			return nil, fmt.Errorf("get moderation for list assets: %w", mErr)
		}
		if moderationState == "delist" || moderationState == "quarantine" {
			continue
		}

		payee, pErr := s.payeesRepo.GetPayee(ctx, assetModel.PayeeID)
		if pErr != nil {
			if errors.Is(pErr, repo.ErrNotFound) {
				return nil, fmt.Errorf("get payee for list assets: %w", ErrNotFound)
			}
			return nil, fmt.Errorf("get payee for list assets: %w", pErr)
		}

		hints, hErr := s.providerHintsRepo.ListHintsByAsset(ctx, assetModel.AssetID)
		if hErr != nil {
			return nil, fmt.Errorf("list hints for list assets: %w", hErr)
		}
		result = append(result, toAsset(assetModel, payee, len(hints), moderationState))
	}
	return result, nil
}

func (s *Service) AddProviderHint(ctx context.Context, assetID string, input CreateProviderHintInput) (ProviderHint, error) {
	_, err := s.assetsRepo.GetAsset(ctx, model.AssetID(assetID))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ProviderHint{}, fmt.Errorf("get asset for add hint: %w", ErrNotFound)
		}
		return ProviderHint{}, fmt.Errorf("get asset for add hint: %w", err)
	}

	created, err := s.providerHintsRepo.AddHint(ctx, repo.AddProviderHintParams{
		HintID:    model.ProviderHintID("hint_" + generateUUID()),
		AssetID:   model.AssetID(assetID),
		Transport: input.Transport,
		BaseURL:   input.BaseURL,
		Priority:  input.Priority,
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return ProviderHint{}, fmt.Errorf("add provider hint conflict: %w", ErrConflict)
		}
		return ProviderHint{}, fmt.Errorf("add provider hint: %w", err)
	}

	return toProviderHint(created), nil
}

func (s *Service) ListProviderHints(ctx context.Context, assetID string) ([]ProviderHint, error) {
	_, err := s.assetsRepo.GetAsset(ctx, model.AssetID(assetID))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, fmt.Errorf("get asset for list hints: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("get asset for list hints: %w", err)
	}

	hints, err := s.providerHintsRepo.ListHintsByAsset(ctx, model.AssetID(assetID))
	if err != nil {
		return nil, fmt.Errorf("list provider hints: %w", err)
	}

	result := make([]ProviderHint, 0, len(hints))
	for _, hint := range hints {
		result = append(result, toProviderHint(hint))
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority == result[j].Priority {
			return result[i].CreatedAt > result[j].CreatedAt
		}
		return result[i].Priority > result[j].Priority
	})

	return result, nil
}

func toAsset(value model.Asset, payee model.Payee, hintsCount int, moderationState string) Asset {
	base := strings.TrimRight(payee.FAPPublicBaseURL, "/")
	return Asset{
		AssetID:       string(value.AssetID),
		ArtistID:      string(value.ArtistID),
		PayeeID:       string(value.PayeeID),
		Title:         value.Title,
		DurationMS:    value.DurationMS,
		ContentID:     value.ContentID,
		HLSMasterURL:  value.HLSMasterURL,
		PreviewHLSURL: value.PreviewHLSURL,
		PriceMSat:     value.PriceMSat,
		CreatedAt:     value.CreatedAt,
		UpdatedAt:     value.UpdatedAt,
		Purchase: Purchase{
			ResourceID:   "hls:key:" + string(value.AssetID),
			ChallengeURL: base + "/v1/fap/challenge",
			TokenURL:     base + "/v1/fap/token",
			PayeeID:      string(payee.PayeeID),
			FAPPayeeID:   payee.FAPPayeeID,
			PriceMSat:    value.PriceMSat,
		},
		Moderation:         Moderation{State: moderationState},
		ProviderHintsCount: hintsCount,
	}
}

func toProviderHint(value model.ProviderHint) ProviderHint {
	return ProviderHint{
		HintID:    string(value.HintID),
		AssetID:   string(value.AssetID),
		Transport: value.Transport,
		BaseURL:   value.BaseURL,
		Priority:  value.Priority,
		CreatedAt: value.CreatedAt,
	}
}

func (s *Service) getModerationState(ctx context.Context, targetType string, targetID string) (string, error) {
	if s.moderationRepo == nil {
		return moderationStateAllow(), nil
	}
	state, exists, err := s.moderationRepo.GetState(ctx, targetType, targetID)
	if err != nil {
		return "", err
	}
	if !exists {
		return moderationStateAllow(), nil
	}
	if state.State == "" {
		return moderationStateAllow(), nil
	}
	return state.State, nil
}

func moderationStateAllow() string {
	return "allow"
}

func isUniqueConstraint(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique || sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "constraint")
}

func generateUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}
