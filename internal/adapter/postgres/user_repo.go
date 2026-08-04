package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/leftmy/planetarium-server/internal/domain"
)

// uniqueViolation is the SQLSTATE Postgres raises for a broken unique
// constraint. See https://www.postgresql.org/docs/current/errcodes-appendix.html
const uniqueViolation = "23505"

// UserRepo reads and writes the users table.
//
// Every query filters on deleted_at IS NULL. That is not optional: the email
// uniqueness index is partial over living rows, so a soft-deleted account can
// share an address with a live one, and a query that forgets the filter would
// authenticate the deleted account.
type UserRepo struct {
	db *DB
}

func NewUserRepo(db *DB) *UserRepo {
	return &UserRepo{db: db}
}

// Create inserts a user. The caller supplies the ID via domain.NewID so that
// it is known before the row exists and can be referenced in the same
// transaction.
//
// A duplicate email comes back as domain.ErrAlreadyExists rather than a
// Postgres error, so callers do not have to know SQLSTATE codes. Relying on the
// constraint instead of a preceding SELECT is also what makes concurrent
// registrations of the same address safe.
func (r *UserRepo) Create(ctx context.Context, u domain.User) error {
	const query = `
		INSERT INTO users (id, email, display_name, password_hash)
		VALUES ($1, $2, $3, NULLIF($4, ''))`

	_, err := r.db.Querier(ctx).Exec(ctx, query, u.ID, string(u.Email), u.Name, u.PasswordHash)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return domain.ErrAlreadyExists
		}

		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

// ByEmail loads a living user by address. Returns domain.ErrNotFound when there
// is none, so callers distinguish "absent" from "query failed".
func (r *UserRepo) ByEmail(ctx context.Context, email domain.Email) (domain.User, error) {
	const query = `
		SELECT id, email, display_name, COALESCE(password_hash, '')
		FROM users
		WHERE email = $1 AND deleted_at IS NULL`

	return r.scanOne(ctx, query, string(email))
}

// ByID loads a living user by primary key.
func (r *UserRepo) ByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	const query = `
		SELECT id, email, display_name, COALESCE(password_hash, '')
		FROM users
		WHERE id = $1 AND deleted_at IS NULL`

	return r.scanOne(ctx, query, id)
}

func (r *UserRepo) scanOne(ctx context.Context, query string, args ...any) (domain.User, error) {
	var (
		u     domain.User
		email string
	)

	err := r.db.Querier(ctx).QueryRow(ctx, query, args...).
		Scan(&u.ID, &email, &u.Name, &u.PasswordHash)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.User{}, domain.ErrNotFound
	case err != nil:
		return domain.User{}, fmt.Errorf("select user: %w", err)
	}

	// The address is already stored normalised, so it does not go through
	// domain.NewEmail again — that would reject rows the database accepted.
	u.Email = domain.Email(email)

	return u, nil
}
