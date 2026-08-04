package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/leftmy/planetarium-server/internal/domain"
)

// RefreshTokenRepo stores issued sessions.
//
// Only hashes go in: a database dump must not hand an attacker a set of live
// sessions. Nothing here compares plaintext, so the repository never sees a
// token at all.
type RefreshTokenRepo struct {
	db *DB
}

func NewRefreshTokenRepo(db *DB) *RefreshTokenRepo {
	return &RefreshTokenRepo{db: db}
}

func (r *RefreshTokenRepo) Create(ctx context.Context, t domain.RefreshToken) error {
	const query = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`

	_, err := r.db.Querier(ctx).Exec(ctx, query, t.ID, t.UserID, t.TokenHash, t.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}

	return nil
}

// ByHash loads a token by its stored hash, revoked or not.
//
// Returning revoked tokens is the point: a caller that only ever saw live rows
// could not tell "never existed" from "already used", and those need very
// different responses.
func (r *RefreshTokenRepo) ByHash(ctx context.Context, tokenHash string) (domain.RefreshToken, error) {
	const query = `
		SELECT id, user_id, token_hash, replaced_by, expires_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1`

	var t domain.RefreshToken

	err := r.db.Querier(ctx).QueryRow(ctx, query, tokenHash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ReplacedBy, &t.ExpiresAt, &t.RevokedAt,
	)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.RefreshToken{}, domain.ErrNotFound
	case err != nil:
		return domain.RefreshToken{}, fmt.Errorf("select refresh token: %w", err)
	}

	return t, nil
}

// Revoke marks a token as used and records which token replaced it.
//
// The WHERE clause refuses to revoke twice. That makes rotation safe under
// concurrency: if two requests present the same token at once, exactly one
// updates a row, and the other sees zero and can treat it as reuse.
func (r *RefreshTokenRepo) Revoke(ctx context.Context, id, replacedBy uuid.UUID) error {
	const query = `
		UPDATE refresh_tokens
		SET revoked_at = now(), replaced_by = $2
		WHERE id = $1 AND revoked_at IS NULL`

	tag, err := r.db.Querier(ctx).Exec(ctx, query, id, replacedBy)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// RevokeChain invalidates every live session of a user.
//
// This is the response to a revoked token being presented again: the token was
// already exchanged, so whoever just sent it holds a copy, and the session it
// was exchanged for cannot be trusted either. Refusing only the current request
// would leave the thief's session running.
func (r *RefreshTokenRepo) RevokeChain(ctx context.Context, userID uuid.UUID) error {
	const query = `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL`

	if _, err := r.db.Querier(ctx).Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("revoke token chain: %w", err)
	}

	return nil
}
