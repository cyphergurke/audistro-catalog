package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"audistro-catalog/internal/model"
	"audistro-catalog/internal/store/repo"
)

type Service struct {
	artistsRepo repo.ArtistsRepository
	payeesRepo  repo.PayeesRepository
}

func NewService(artistsRepo repo.ArtistsRepository, payeesRepo repo.PayeesRepository) (*Service, error) {
	if artistsRepo == nil {
		return nil, errors.New("artists repo is required")
	}
	if payeesRepo == nil {
		return nil, errors.New("payees repo is required")
	}
	return &Service{artistsRepo: artistsRepo, payeesRepo: payeesRepo}, nil
}

func (s *Service) BootstrapArtist(ctx context.Context, input BootstrapArtistInput) (BootstrapArtistResult, error) {
	handle := strings.TrimSpace(input.Handle)
	artistID := strings.TrimSpace(input.ArtistID)
	pubKeyHex := strings.TrimSpace(input.PubKeyHex)
	displayName := strings.TrimSpace(input.DisplayName)
	payeeID := strings.TrimSpace(input.Payee.PayeeID)
	fapPublicBaseURL := strings.TrimSpace(input.Payee.FAPPublicBaseURL)
	fapPayeeID := strings.TrimSpace(input.Payee.FAPPayeeID)

	if artistID == "" {
		artistID = deterministicID("ar", handle)
	}
	if pubKeyHex == "" {
		pubKeyHex = deterministicHex(handle)
	}
	if payeeID == "" {
		payeeID = deterministicID("pe", handle+"\n"+fapPayeeID)
	}

	artist, err := s.resolveOrCreateArtist(ctx, artistID, handle, displayName, pubKeyHex)
	if err != nil {
		return BootstrapArtistResult{}, err
	}
	payee, err := s.resolveOrCreatePayee(ctx, artist.ArtistID, payeeID, fapPublicBaseURL, fapPayeeID)
	if err != nil {
		return BootstrapArtistResult{}, err
	}

	return BootstrapArtistResult{
		ArtistID:   string(artist.ArtistID),
		PayeeID:    string(payee.PayeeID),
		Handle:     artist.Handle,
		FAPPayeeID: payee.FAPPayeeID,
	}, nil
}

func (s *Service) resolveOrCreateArtist(ctx context.Context, artistID string, handle string, displayName string, pubKeyHex string) (model.Artist, error) {
	byHandle, err := s.artistsRepo.GetArtistByHandle(ctx, handle)
	if err == nil {
		if artistID != "" && string(byHandle.ArtistID) != artistID {
			return model.Artist{}, fmt.Errorf("artist handle conflict: %w", ErrConflict)
		}
		return byHandle, nil
	}
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return model.Artist{}, fmt.Errorf("lookup artist by handle: %w", err)
	}

	byID, err := s.artistsRepo.GetArtistByID(ctx, model.ArtistID(artistID))
	if err == nil {
		if byID.Handle != handle {
			return model.Artist{}, fmt.Errorf("artist id conflict: %w", ErrConflict)
		}
		return byID, nil
	}
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return model.Artist{}, fmt.Errorf("lookup artist by id: %w", err)
	}

	created, err := s.artistsRepo.CreateArtist(ctx, repo.CreateArtistParams{
		ArtistID:    model.ArtistID(artistID),
		PubKeyHex:   pubKeyHex,
		Handle:      handle,
		DisplayName: displayName,
		Bio:         "",
		AvatarURL:   "",
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return model.Artist{}, fmt.Errorf("create artist: %w", ErrConflict)
		}
		return model.Artist{}, fmt.Errorf("create artist: %w", err)
	}
	return created, nil
}

func (s *Service) resolveOrCreatePayee(ctx context.Context, artistID model.ArtistID, payeeID string, fapPublicBaseURL string, fapPayeeID string) (model.Payee, error) {
	if payeeID != "" {
		byID, err := s.payeesRepo.GetPayee(ctx, model.PayeeID(payeeID))
		if err == nil {
			if byID.ArtistID != artistID || byID.FAPPayeeID != fapPayeeID || byID.FAPPublicBaseURL != fapPublicBaseURL {
				return model.Payee{}, fmt.Errorf("payee id conflict: %w", ErrConflict)
			}
			return byID, nil
		}
		if err != nil && !errors.Is(err, repo.ErrNotFound) {
			return model.Payee{}, fmt.Errorf("lookup payee by id: %w", err)
		}
	}

	payees, err := s.payeesRepo.ListPayeesByArtist(ctx, artistID)
	if err != nil {
		return model.Payee{}, fmt.Errorf("list payees by artist: %w", err)
	}
	for _, payee := range payees {
		if payee.FAPPayeeID != fapPayeeID {
			continue
		}
		if payeeID != "" && string(payee.PayeeID) != payeeID {
			return model.Payee{}, fmt.Errorf("payee mapping conflict: %w", ErrConflict)
		}
		if payee.FAPPublicBaseURL != fapPublicBaseURL {
			return model.Payee{}, fmt.Errorf("payee mapping conflict: %w", ErrConflict)
		}
		return payee, nil
	}

	created, err := s.payeesRepo.CreatePayee(ctx, repo.CreatePayeeParams{
		PayeeID:          model.PayeeID(payeeID),
		ArtistID:         artistID,
		FAPPublicBaseURL: fapPublicBaseURL,
		FAPPayeeID:       fapPayeeID,
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return model.Payee{}, fmt.Errorf("create payee: %w", ErrConflict)
		}
		return model.Payee{}, fmt.Errorf("create payee: %w", err)
	}
	return created, nil
}

func deterministicID(prefix string, seed string) string {
	digest := deterministicHex(seed)
	return prefix + "_" + digest[:20]
}

func deterministicHex(seed string) string {
	hash := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(hash[:])
}

func isUniqueConstraint(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "constraint")
}
