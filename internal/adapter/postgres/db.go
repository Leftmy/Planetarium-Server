package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config contains only what relevant to database
type Config struct {
	User     string `env:"DB_USER"`
	Password string `env:"DB_PASSWORD"`
	Host     string `env:"DB_HOST"`
	Port     uint   `env:"DB_PORT"`
	Name     string `env:"DB_NAME"`
}

func (c *Config) BuildURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.User, c.Password, c.Host, c.Port, c.Name,
	)
}

// Querier is the subset of pgx that repositories use. Both *pgxpool.Pool and
// pgx.Tx satisfy it, which is what lets the same repository code run inside a
// transaction and outside one.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// DB owns the connection pool and hands repositories the right querier.
type DB struct {
	pool *pgxpool.Pool
}

// New opens the pool and verifies that the database actually answers, so a
// misconfigured DSN fails at startup rather than on the first request.
func New(ctx context.Context, cfg Config) (*DB, error) {
	pool, err := pgxpool.New(ctx, cfg.BuildURL())
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

// txKey carries the active transaction. It is unexported and of a private type
// so nothing outside this package can put a value under it.
type txKey struct{}

// Querier returns the transaction bound to ctx, or the pool when there is none.
// Repositories call this instead of touching the pool directly.
func (db *DB) Querier(ctx context.Context) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}

	return db.pool
}

// WithinTx runs fn inside a single transaction, committing when fn returns nil
// and rolling back otherwise. The transaction travels in the context, so every
// repository called underneath joins it without being passed anything extra.
//
// Calls nest safely: an inner WithinTx joins the transaction already in ctx
// instead of opening a second one, which would block against the first.
func (db *DB) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		// The rollback error is deliberately dropped: fn's error is the one
		// that explains the failure, and rollback typically fails only because
		// the transaction is already gone.
		_ = tx.Rollback(ctx)

		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}
