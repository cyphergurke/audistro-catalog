package payees

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	sqlite3 "github.com/mattn/go-sqlite3"
	"audistro-catalog/internal/model"
	"audistro-catalog/internal/store/repo"
)

type Service struct {
	payeesRepo  repo.PayeesRepository
	artistsRepo repo.ArtistsRepository
}

func NewService(payeesRepo repo.PayeesRepository, artistsRepo repo.ArtistsRepository) *Service {
	return &Service{
		payeesRepo:  payeesRepo,
		artistsRepo: artistsRepo,
	}
}

func (s *Service) CreatePayee(ctx context.Context, input CreatePayeeInput) (Payee, error) {
	artist, err := s.artistsRepo.GetArtistByHandle(ctx, input.ArtistHandle)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return Payee{}, fmt.Errorf("get artist for payee: %w", ErrNotFound)
		}
		return Payee{}, fmt.Errorf("get artist for payee: %w", err)
	}

	created, err := s.payeesRepo.CreatePayee(ctx, repo.CreatePayeeParams{
		PayeeID:          model.PayeeID(generateID("pay")),
		ArtistID:         artist.ArtistID,
		FAPPublicBaseURL: input.FAPPublicBaseURL,
		FAPPayeeID:       input.FAPPayeeID,
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return Payee{}, fmt.Errorf("create payee: %w", ErrConflict)
		}
		return Payee{}, fmt.Errorf("create payee: %w", err)
	}

	return toPayee(created), nil
}

func (s *Service) GetPayee(ctx context.Context, payeeID string) (Payee, error) {
	value, err := s.payeesRepo.GetPayee(ctx, model.PayeeID(payeeID))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return Payee{}, fmt.Errorf("get payee: %w", ErrNotFound)
		}
		return Payee{}, fmt.Errorf("get payee: %w", err)
	}
	return toPayee(value), nil
}

func (s *Service) ListPayeesByArtistHandle(ctx context.Context, handle string) ([]Payee, error) {
	artist, err := s.artistsRepo.GetArtistByHandle(ctx, handle)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, fmt.Errorf("get artist for list payees: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("get artist for list payees: %w", err)
	}

	values, err := s.payeesRepo.ListPayeesByArtist(ctx, artist.ArtistID)
	if err != nil {
		return nil, fmt.Errorf("list payees by artist: %w", err)
	}

	result := make([]Payee, 0, len(values))
	for _, value := range values {
		result = append(result, toPayee(value))
	}
	return result, nil
}

func toPayee(value model.Payee) Payee {
	return Payee{
		PayeeID:          string(value.PayeeID),
		ArtistID:         string(value.ArtistID),
		FAPPublicBaseURL: value.FAPPublicBaseURL,
		FAPPayeeID:       value.FAPPayeeID,
		CreatedAt:        value.CreatedAt,
		UpdatedAt:        value.UpdatedAt,
	}
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
