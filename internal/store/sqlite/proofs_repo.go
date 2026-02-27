package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cyphergurke/audistro-catalog/internal/model"
	"github.com/cyphergurke/audistro-catalog/internal/store/repo"
)

var _ repo.ProofsRepository = (*ProofsRepo)(nil)

type ProofsRepo struct {
	db *sql.DB
}

func NewProofsRepo(db *sql.DB) *ProofsRepo {
	return &ProofsRepo{db: db}
}

func (r *ProofsRepo) ListByStatus(ctx context.Context, params repo.ListProofsByStatusParams) ([]model.ArtistProof, error) {
	if len(params.Statuses) == 0 {
		return []model.ArtistProof{}, nil
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	placeholders := make([]string, len(params.Statuses))
	args := make([]any, 0, len(params.Statuses)+1)
	for i, status := range params.Statuses {
		placeholders[i] = "?"
		args = append(args, string(status))
	}
	args = append(args, limit)

	query := `
		SELECT proof_id, artist_pubkey_hex, proof_type, proof_value, status, checked_at, details, created_at, updated_at
		FROM artist_proofs
		WHERE status IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY updated_at ASC, proof_id ASC
		LIMIT ?
	`

	stmt, err := r.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("prepare list proofs by status: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("list proofs by status: %w", err)
	}
	defer rows.Close()

	proofs := make([]model.ArtistProof, 0)
	for rows.Next() {
		proof, scanErr := scanProof(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan proof row: %w", scanErr)
		}
		proofs = append(proofs, proof)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proof rows: %w", err)
	}

	return proofs, nil
}

func (r *ProofsRepo) UpdateStatus(ctx context.Context, params repo.UpdateProofStatusParams) error {
	updatedAt := params.UpdatedAt
	if updatedAt == 0 {
		updatedAt = time.Now().Unix()
	}

	stmt, err := r.db.PrepareContext(ctx, `
		UPDATE artist_proofs
		SET status = ?, checked_at = ?, details = ?, updated_at = ?
		WHERE proof_id = ?
	`)
	if err != nil {
		return fmt.Errorf("prepare update proof status: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, string(params.Status), params.CheckedAt, params.Details, updatedAt, params.ProofID)
	if err != nil {
		return fmt.Errorf("update proof status: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated proof rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("update proof status: %w", repo.ErrNotFound)
	}

	return nil
}

func (r *ProofsRepo) ListByArtistPubKey(ctx context.Context, pubKeyHex string) ([]model.ArtistProof, error) {
	stmt, err := r.db.PrepareContext(ctx, `
		SELECT proof_id, artist_pubkey_hex, proof_type, proof_value, status, checked_at, details, created_at, updated_at
		FROM artist_proofs
		WHERE artist_pubkey_hex = ?
		ORDER BY proof_type ASC, proof_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare list proofs by artist pubkey: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, pubKeyHex)
	if err != nil {
		return nil, fmt.Errorf("list proofs by artist pubkey: %w", err)
	}
	defer rows.Close()

	proofs := make([]model.ArtistProof, 0)
	for rows.Next() {
		proof, scanErr := scanProof(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan proof row: %w", scanErr)
		}
		proofs = append(proofs, proof)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proof rows: %w", err)
	}

	return proofs, nil
}

func scanProof(scanner interface {
	Scan(dest ...any) error
}) (model.ArtistProof, error) {
	var proof model.ArtistProof
	var proofType string
	var status string
	if err := scanner.Scan(
		&proof.ProofID,
		&proof.ArtistPubKeyHex,
		&proofType,
		&proof.ProofValue,
		&status,
		&proof.CheckedAt,
		&proof.Details,
		&proof.CreatedAt,
		&proof.UpdatedAt,
	); err != nil {
		return model.ArtistProof{}, err
	}
	proof.ProofType = model.ProofType(proofType)
	proof.Status = model.ProofStatus(status)
	return proof, nil
}
