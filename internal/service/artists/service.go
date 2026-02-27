package artists

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/cyphergurke/audistro-catalog/internal/model"
	"github.com/cyphergurke/audistro-catalog/internal/store/repo"
)

type Service struct {
	artistsRepo      repo.ArtistsRepository
	moderationRepo   repo.ModerationRepository
	verificationRepo repo.VerificationRepository
}

func NewService(
	artistsRepo repo.ArtistsRepository,
	moderationRepo repo.ModerationRepository,
	verificationRepo repo.VerificationRepository,
) *Service {
	return &Service{
		artistsRepo:      artistsRepo,
		moderationRepo:   moderationRepo,
		verificationRepo: verificationRepo,
	}
}

func (s *Service) CreateArtist(ctx context.Context, input CreateArtistInput) (Artist, error) {
	created, err := s.artistsRepo.CreateArtist(ctx, repo.CreateArtistParams{
		ArtistID:    model.ArtistID(generateID("art")),
		PubKeyHex:   input.PubKeyHex,
		Handle:      input.Handle,
		DisplayName: input.DisplayName,
		Bio:         input.Bio,
		AvatarURL:   input.AvatarURL,
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return Artist{}, fmt.Errorf("create artist: %w", ErrConflict)
		}
		return Artist{}, fmt.Errorf("create artist: %w", err)
	}

	artist, err := s.buildArtist(ctx, created)
	if err != nil {
		return Artist{}, fmt.Errorf("create artist response: %w", err)
	}
	return artist, nil
}

func (s *Service) GetArtistByHandle(ctx context.Context, handle string) (Artist, error) {
	artistModel, err := s.artistsRepo.GetArtistByHandle(ctx, handle)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return Artist{}, fmt.Errorf("get artist by handle: %w", ErrNotFound)
		}
		return Artist{}, fmt.Errorf("get artist by handle: %w", err)
	}

	artist, err := s.buildArtist(ctx, artistModel)
	if err != nil {
		return Artist{}, fmt.Errorf("get artist by handle response: %w", err)
	}
	return artist, nil
}

func (s *Service) ListArtists(ctx context.Context) ([]Artist, error) {
	values, err := s.artistsRepo.ListArtists(ctx)
	if err != nil {
		return nil, fmt.Errorf("list artists: %w", err)
	}

	result := make([]Artist, 0, len(values))
	for _, value := range values {
		artist, buildErr := s.buildArtist(ctx, value)
		if buildErr != nil {
			return nil, fmt.Errorf("build artist in list: %w", buildErr)
		}
		result = append(result, artist)
	}
	return result, nil
}

func (s *Service) buildArtist(ctx context.Context, value model.Artist) (Artist, error) {
	moderationState := "allow"
	if s.moderationRepo != nil {
		moderation, exists, err := s.moderationRepo.GetState(ctx, "artist", string(value.ArtistID))
		if err != nil {
			return Artist{}, fmt.Errorf("get moderation state: %w", err)
		}
		if exists && moderation.State != "" {
			moderationState = moderation.State
		}
	}

	verification := Verification{Badge: "unverified", Score: 0}
	if s.verificationRepo != nil {
		state, err := s.verificationRepo.GetByPubKeyHex(ctx, value.PubKeyHex)
		if err != nil {
			if !errors.Is(err, repo.ErrNotFound) {
				return Artist{}, fmt.Errorf("get verification state: %w", err)
			}
		} else {
			verification.Badge = state.Badge
			verification.Score = int(state.Score)
		}
	}

	return Artist{
		ArtistID:     string(value.ArtistID),
		PubKeyHex:    value.PubKeyHex,
		Handle:       value.Handle,
		DisplayName:  value.DisplayName,
		Bio:          value.Bio,
		AvatarURL:    value.AvatarURL,
		CreatedAt:    value.CreatedAt,
		UpdatedAt:    value.UpdatedAt,
		Verification: verification,
		Moderation:   Moderation{State: moderationState},
	}, nil
}

func isUniqueConstraint(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique || sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "constraint")
}

func generateID(prefix string) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err == nil {
		return prefix + "_" + hex.EncodeToString(buf)
	}
	return prefix + "_fallback"
}
