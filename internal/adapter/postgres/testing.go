package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/leftmy/planetarium-server/migrations"
)

// testImage matches the image docker-compose.yml runs, so a test never passes
// against a Postgres version the application will not meet in development.
const testImage = "postgres:16"

// startOnce guards the shared container: it is started on the first NewTestDB
// call in a test binary and reused by every later one. Starting a container per
// test would add seconds to each of them, which is what makes teams stop writing
// database tests at all.
var (
	startOnce sync.Once
	sharedDSN string
	startErr  error
)

// NewTestDB returns a DB pointed at a PostgreSQL instance with the migrations
// applied, and empties every table first so each test starts from a known state.
//
// The database comes from one of two places:
//
//   - TEST_DATABASE_URL, when set — used as is. CI sets it to a service
//     container, which avoids paying for Docker-in-Docker inside the runner.
//   - otherwise a throwaway container started through testcontainers-go and
//     removed when the test binary exits.
//
// Tests that call this need a working Docker daemon. Under `go test -short`
// they skip instead of failing, so `go test -short ./...` stays useful on a
// machine without Docker.
func NewTestDB(t *testing.T) *DB {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping database test: -short")
	}

	startOnce.Do(func() { sharedDSN, startErr = startPostgres() })

	if startErr != nil {
		t.Fatalf("start test database: %v", startErr)
	}

	ctx := context.Background()

	pool, err := New(ctx, dsnConfig(sharedDSN))
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}

	t.Cleanup(pool.Close)

	if err := truncateAll(ctx, pool); err != nil {
		t.Fatalf("reset test database: %v", err)
	}

	return pool
}

// TestDSN returns the connection string of the shared test database, already
// migrated. Tests that need a raw *sql.DB rather than the pool — the migration
// test is the one case — use this instead of NewTestDB.
//
// A test that migrates the schema down must migrate it back up before it
// returns, since everything else in the package shares this database.
func TestDSN(t *testing.T) string {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping database test: -short")
	}

	startOnce.Do(func() { sharedDSN, startErr = startPostgres() })

	if startErr != nil {
		t.Fatalf("start test database: %v", startErr)
	}

	return sharedDSN
}

// startPostgres brings up the database and migrates it. It runs at most once per
// test binary.
func startPostgres() (string, error) {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		own, err := packageDatabase(dsn)
		if err != nil {
			return "", fmt.Errorf("prepare package database: %w", err)
		}

		if err := migrate(own); err != nil {
			return "", fmt.Errorf("migrate TEST_DATABASE_URL: %w", err)
		}

		return own, nil
	}

	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, testImage,
		tcpostgres.WithDatabase("planetarium_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("secret"),
		// Postgres logs "ready to accept connections" once while initialising
		// the data directory and again once it is actually listening, so waiting
		// for a single occurrence connects too early.
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(2*time.Minute),
		),
	)
	if err != nil {
		return "", fmt.Errorf("start container: %w", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return "", fmt.Errorf("connection string: %w", err)
	}

	if err := migrate(dsn); err != nil {
		return "", fmt.Errorf("migrate container: %w", err)
	}

	return dsn, nil
}

// packageDatabase gives this test binary a database of its own on the server
// TEST_DATABASE_URL points at, creating it on first use, and returns the DSN for
// it.
//
// `go test ./...` runs packages in parallel. Without this they would all share
// one database and truncate each other's rows mid-test, which fails in ways that
// look like application bugs and disappear when a package is run alone. The
// container path does not need it: each test binary starts its own container.
//
// The name is derived from the binary rather than random, so repeated runs reuse
// the same database instead of leaving a trail of them behind.
func packageDatabase(adminDSN string) (string, error) {
	u, err := url.Parse(adminDSN)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}

	name := "planetarium_test_" + sanitiseDBName(filepath.Base(os.Args[0]))

	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		return "", fmt.Errorf("open admin connection: %w", err)
	}
	defer func() { _ = admin.Close() }()

	// CREATE DATABASE has no IF NOT EXISTS, and the name cannot be a parameter,
	// hence the quoted identifier and the tolerated duplicate error.
	_, err = admin.Exec(`CREATE DATABASE "` + name + `"`)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return "", fmt.Errorf("create database %q: %w", name, err)
	}

	u.Path = "/" + name

	return u.String(), nil
}

// sanitiseDBName turns a test binary's file name into something safe to use as
// an identifier: "postgres.test.exe" becomes "postgres".
func sanitiseDBName(binary string) string {
	name := strings.TrimSuffix(binary, ".exe")
	name = strings.TrimSuffix(name, ".test")

	var b strings.Builder

	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}

	if b.Len() == 0 {
		return "unnamed"
	}

	return b.String()
}

// migrate applies the same embedded migrations the migrate command runs, so the
// tested schema is the deployed schema rather than a fixture that drifts from it.
func migrate(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("up: %w", err)
	}

	return nil
}

// dsnConfig wraps a ready DSN so it can travel through New, which takes the
// application's Config rather than a URL.
func dsnConfig(dsn string) Config {
	return Config{url: dsn}
}

// truncateAll empties every table except goose's bookkeeping. TRUNCATE ... CASCADE
// in one statement handles the foreign keys between them, and RESTART IDENTITY
// resets the sequences the schema does use.
func truncateAll(ctx context.Context, db *DB) error {
	const query = `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_type = 'BASE TABLE'
		  AND table_name <> 'goose_db_version'`

	rows, err := db.Querier(ctx).Query(ctx, query)
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	var tables []string

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan table name: %w", err)
		}

		tables = append(tables, `"`+name+`"`)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tables: %w", err)
	}

	if len(tables) == 0 {
		return nil
	}

	stmt := "TRUNCATE " + strings.Join(tables, ", ") + " RESTART IDENTITY CASCADE"
	if _, err := db.Querier(ctx).Exec(ctx, stmt); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	return nil
}
