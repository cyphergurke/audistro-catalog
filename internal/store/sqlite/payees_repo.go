package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cyphergurke/audistro-catalog/internal/model"
	"github.com/cyphergurke/audistro-catalog/internal/store/repo"
)

var _ repo.PayeesRepository = (*PayeesRepo)(nil)

type PayeesRepo struct {
	db *sql.DB
}

func NewPayeesRepo(db *sql.DB) *PayeesRepo {
	return &PayeesRepo{db: db}
}

func (r *PayeesRepo) CreatePayee(ctx context.Context, params repo.CreatePayeeParams) (model.Payee, error) {
	now := time.Now().Unix()
	payee := model.Payee{
		PayeeID:          params.PayeeID,
		ArtistID:         params.ArtistID,
		FAPPublicBaseURL: params.FAPPublicBaseURL,
		FAPPayeeID:       params.FAPPayeeID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	stmt, err := r.db.PrepareContext(ctx, `
		INSERT INTO payees(
			payee_id, artist_id, fap_public_base_url, fap_payee_id, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return model.Payee{}, fmt.Errorf("prepare create payee: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(
		ctx,
		string(payee.PayeeID),
		string(payee.ArtistID),
		payee.FAPPublicBaseURL,
		payee.FAPPayeeID,
		payee.CreatedAt,
		payee.UpdatedAt,
	)
	if err != nil {
		return model.Payee{}, fmt.Errorf("create payee: %w", err)
	}

	return payee, nil
}

func (r *PayeesRepo) GetPayee(ctx context.Context, payeeID model.PayeeID) (model.Payee, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT payee_id, artist_id, fap_public_base_url, fap_payee_id, created_at, updated_at
		FROM payees WHERE payee_id = ?
	`)
	if err != nil {
		return model.Payee{}, fmt.Errorf("prepare get payee: %w", err)
	}
	defer stmt.Close()

	payee, err := scanPayee(stmt.QueryRowContext(ctx, string(payeeID)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Payee{}, fmt.Errorf("get payee: %w", repo.ErrNotFound)
		}
		return model.Payee{}, fmt.Errorf("get payee: %w", err)
	}
	return payee, nil
}

func (r *PayeesRepo) ListPayeesByArtist(ctx context.Context, artistID model.ArtistID) ([]model.Payee, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT payee_id, artist_id, fap_public_base_url, fap_payee_id, created_at, updated_at
		FROM payees WHERE artist_id = ? ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare list payees by artist: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, string(artistID))
	if err != nil {
		return nil, fmt.Errorf("list payees by artist: %w", err)
	}
	defer rows.Close()

	payees := make([]model.Payee, 0)
	for rows.Next() {
		var payee model.Payee
		var payeeIDRaw string
		var artistIDRaw string
		if err := rows.Scan(
			&payeeIDRaw,
			&artistIDRaw,
			&payee.FAPPublicBaseURL,
			&payee.FAPPayeeID,
			&payee.CreatedAt,
			&payee.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan payee row: %w", err)
		}
		payee.PayeeID = model.PayeeID(payeeIDRaw)
		payee.ArtistID = model.ArtistID(artistIDRaw)
		payees = append(payees, payee)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate payees rows: %w", err)
	}

	return payees, nil
}

func scanPayee(row *sql.Row) (model.Payee, error) {
	var payee model.Payee
	var payeeIDRaw string
	var artistIDRaw string
	if err := row.Scan(
		&payeeIDRaw,
		&artistIDRaw,
		&payee.FAPPublicBaseURL,
		&payee.FAPPayeeID,
		&payee.CreatedAt,
		&payee.UpdatedAt,
	); err != nil {
		return model.Payee{}, err
	}
	payee.PayeeID = model.PayeeID(payeeIDRaw)
	payee.ArtistID = model.ArtistID(artistIDRaw)
	return payee, nil
}
