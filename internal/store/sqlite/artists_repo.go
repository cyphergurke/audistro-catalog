package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"audistro-catalog/internal/model"
	"audistro-catalog/internal/store/repo"
)

var _ repo.ArtistsRepository = (*ArtistsRepo)(nil)

type ArtistsRepo struct {
	db *sql.DB
}

func NewArtistsRepo(db *sql.DB) *ArtistsRepo {
	return &ArtistsRepo{db: db}
}

func (r *ArtistsRepo) CreateArtist(ctx context.Context, params repo.CreateArtistParams) (model.Artist, error) {
	now := time.Now().Unix()
	artist := model.Artist{
		ArtistID:    params.ArtistID,
		PubKeyHex:   params.PubKeyHex,
		Handle:      params.Handle,
		DisplayName: params.DisplayName,
		Bio:         params.Bio,
		AvatarURL:   params.AvatarURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	stmt, err := r.db.PrepareContext(ctx, `
		INSERT INTO artists(
			artist_id, pubkey_hex, handle, display_name, bio, avatar_url, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return model.Artist{}, fmt.Errorf("prepare create artist: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(
		ctx,
		string(artist.ArtistID),
		artist.PubKeyHex,
		artist.Handle,
		artist.DisplayName,
		artist.Bio,
		artist.AvatarURL,
		artist.CreatedAt,
		artist.UpdatedAt,
	)
	if err != nil {
		return model.Artist{}, fmt.Errorf("create artist: %w", err)
	}

	return artist, nil
}

func (r *ArtistsRepo) GetArtistByHandle(ctx context.Context, handle string) (model.Artist, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT artist_id, pubkey_hex, handle, display_name, bio, avatar_url, created_at, updated_at
		FROM artists WHERE handle = ?
	`)
	if err != nil {
		return model.Artist{}, fmt.Errorf("prepare get artist by handle: %w", err)
	}
	defer stmt.Close()

	artist, err := scanArtist(stmt.QueryRowContext(ctx, handle))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Artist{}, fmt.Errorf("get artist by handle: %w", repo.ErrNotFound)
		}
		return model.Artist{}, fmt.Errorf("get artist by handle: %w", err)
	}
	return artist, nil
}

func (r *ArtistsRepo) GetArtistByID(ctx context.Context, artistID model.ArtistID) (model.Artist, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT artist_id, pubkey_hex, handle, display_name, bio, avatar_url, created_at, updated_at
		FROM artists WHERE artist_id = ?
	`)
	if err != nil {
		return model.Artist{}, fmt.Errorf("prepare get artist by id: %w", err)
	}
	defer stmt.Close()

	artist, err := scanArtist(stmt.QueryRowContext(ctx, string(artistID)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Artist{}, fmt.Errorf("get artist by id: %w", repo.ErrNotFound)
		}
		return model.Artist{}, fmt.Errorf("get artist by id: %w", err)
	}
	return artist, nil
}

func (r *ArtistsRepo) GetArtistByPubKey(ctx context.Context, pubKeyHex string) (model.Artist, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT artist_id, pubkey_hex, handle, display_name, bio, avatar_url, created_at, updated_at
		FROM artists WHERE pubkey_hex = ?
	`)
	if err != nil {
		return model.Artist{}, fmt.Errorf("prepare get artist by pubkey: %w", err)
	}
	defer stmt.Close()

	artist, err := scanArtist(stmt.QueryRowContext(ctx, pubKeyHex))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Artist{}, fmt.Errorf("get artist by pubkey: %w", repo.ErrNotFound)
		}
		return model.Artist{}, fmt.Errorf("get artist by pubkey: %w", err)
	}
	return artist, nil
}

func (r *ArtistsRepo) ListArtists(ctx context.Context) ([]model.Artist, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT artist_id, pubkey_hex, handle, display_name, bio, avatar_url, created_at, updated_at
		FROM artists ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare list artists: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list artists: %w", err)
	}
	defer rows.Close()

	artists := make([]model.Artist, 0)
	for rows.Next() {
		var artist model.Artist
		var artistID string
		if err := rows.Scan(
			&artistID,
			&artist.PubKeyHex,
			&artist.Handle,
			&artist.DisplayName,
			&artist.Bio,
			&artist.AvatarURL,
			&artist.CreatedAt,
			&artist.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan artist row: %w", err)
		}
		artist.ArtistID = model.ArtistID(artistID)
		artists = append(artists, artist)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate artist rows: %w", err)
	}

	return artists, nil
}

func scanArtist(row *sql.Row) (model.Artist, error) {
	var artist model.Artist
	var artistID string
	if err := row.Scan(
		&artistID,
		&artist.PubKeyHex,
		&artist.Handle,
		&artist.DisplayName,
		&artist.Bio,
		&artist.AvatarURL,
		&artist.CreatedAt,
		&artist.UpdatedAt,
	); err != nil {
		return model.Artist{}, err
	}
	artist.ArtistID = model.ArtistID(artistID)
	return artist, nil
}
