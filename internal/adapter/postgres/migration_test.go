package postgres_test

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/leftmy/planetarium-server/internal/adapter/postgres"
	"github.com/leftmy/planetarium-server/migrations"
)

// domainTables is what 0001_init.sql creates, excluding goose's own bookkeeping
// table. Listing them explicitly rather than counting rows means a migration that
// forgets to drop one specific table is named in the failure.
var domainTables = []string{
	"achievement_types",
	"categories",
	"node_dependencies",
	"node_progress",
	"node_resources",
	"quiz_attempt_questions",
	"quiz_attempts",
	"quiz_questions",
	"refresh_tokens",
	"roadmap_nodes",
	"roadmaps",
	"skill_node_dependencies",
	"skill_node_states",
	"skill_nodes",
	"user_achievements",
	"user_activity_days",
	"user_identities",
	"user_roadmaps",
	"user_tokens",
	"users",
}

// TestMigrationsUpDownUp is the automated form of the manual check that the
// schema can be rolled back. A down migration that drops things in the wrong
// order, or forgets an object, otherwise stays broken until the day someone
// urgently needs to roll production back.
func TestMigrationsUpDownUp(t *testing.T) {
	db := openMigrationDB(t)

	// The shared database is migrated up already; leave it that way whatever
	// happens below, so the other tests in this package still find a schema.
	t.Cleanup(func() {
		if err := goose.Up(db, "."); err != nil {
			t.Errorf("restore schema: %v", err)
		}
	})

	// Two full cycles: the first proves down works, the second proves down left
	// the database clean enough for up to run again.
	for cycle := 1; cycle <= 2; cycle++ {
		if err := goose.Down(db, "."); err != nil {
			t.Fatalf("cycle %d: down: %v", cycle, err)
		}

		for _, table := range domainTables {
			if tableExists(t, db, table) {
				t.Errorf("cycle %d: table %q survived the down migration", cycle, table)
			}
		}

		if err := goose.Up(db, "."); err != nil {
			t.Fatalf("cycle %d: up: %v", cycle, err)
		}

		for _, table := range domainTables {
			if !tableExists(t, db, table) {
				t.Errorf("cycle %d: table %q missing after the up migration", cycle, table)
			}
		}
	}
}

// TestMigrationsCreateExtensions guards the two extensions the schema depends on:
// citext for case-insensitive email and pg_trgm for the catalogue search indexes.
// Without them the migration fails at a later, less obvious statement.
func TestMigrationsCreateExtensions(t *testing.T) {
	db := openMigrationDB(t)

	for _, ext := range []string{"citext", "pg_trgm"} {
		var present bool

		if err := db.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = $1)`, ext,
		).Scan(&present); err != nil {
			t.Fatalf("query extension %q: %v", ext, err)
		}

		if !present {
			t.Errorf("extension %q is not installed", ext)
		}
	}
}

func openMigrationDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("pgx", postgres.TestDSN(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set dialect: %v", err)
	}

	return db
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()

	var exists bool

	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)`, name).Scan(&exists)
	if err != nil {
		t.Fatalf("check table %q: %v", name, err)
	}

	return exists
}
