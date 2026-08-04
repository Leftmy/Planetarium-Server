package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/leftmy/planetarium-server/internal/domain"
)

// UserIdentityRepo reads and writes user_identities: the rows that say how an
// account can be signed in to.
//
// A password registration writes one row with provider 'local'. Connecting an
// OAuth provider later adds another row for the same user rather than replacing
// it, which is why the table exists separately from users.
type UserIdentityRepo struct {
	db *DB
}

func NewUserIdentityRepo(db *DB) *UserIdentityRepo {
	return &UserIdentityRepo{db: db}
}

// Create inserts an identity.
//
// The UNIQUE (provider, provider_user_id) constraint comes back as
// domain.ErrAlreadyExists: it means this provider account is already attached to
// someone, which is a conflict the caller has to answer for, not a broken query.
func (r *UserIdentityRepo) Create(ctx context.Context, identity domain.UserIdentity) error {
	const query = `
		INSERT INTO user_identities (id, user_id, provider, provider_user_id, email)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))`

	_, err := r.db.Querier(ctx).Exec(ctx, query,
		identity.ID, identity.UserID, identity.Provider, identity.ProviderUserID, identity.Email)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return domain.ErrAlreadyExists
		}

		return fmt.Errorf("insert user identity: %w", err)
	}

	return nil
}

// ByProvider finds the identity a provider account maps to. Returns
// domain.ErrNotFound when the provider account is unknown, which for OAuth is
// the signal to register rather than to sign in.
func (r *UserIdentityRepo) ByProvider(ctx context.Context, provider, providerUserID string) (domain.UserIdentity, error) {
	const query = `
		SELECT id, user_id, provider, provider_user_id, COALESCE(email, '')
		FROM user_identities
		WHERE provider = $1 AND provider_user_id = $2`

	var identity domain.UserIdentity

	err := r.db.Querier(ctx).QueryRow(ctx, query, provider, providerUserID).Scan(
		&identity.ID, &identity.UserID, &identity.Provider,
		&identity.ProviderUserID, &identity.Email,
	)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.UserIdentity{}, domain.ErrNotFound
	case err != nil:
		return domain.UserIdentity{}, fmt.Errorf("select user identity: %w", err)
	}

	return identity, nil
}
