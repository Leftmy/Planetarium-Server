package config_test

import (
	"testing"

	"github.com/leftmy/planetarium-server/config"
)

func TestInitConfigDefaultsMatchEnvExample(t *testing.T) {
	// Cleared explicitly: an empty value falls back to the default, so this
	// stays honest on a machine that already exports DB_* for local work.
	for _, key := range []string{"APP_PORT", "DB_USER", "DB_PASSWORD", "DB_HOST", "DB_PORT", "DB_NAME"} {
		t.Setenv(key, "")
	}

	cfg, err := config.InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}

	if cfg.AppPort != "8080" {
		t.Errorf("AppPort = %q, want 8080", cfg.AppPort)
	}

	if got := cfg.Postgres.BuildURL(); got != "postgres://postgres:secret@localhost:5432/planetarium_db?sslmode=disable" {
		t.Errorf("BuildURL() = %q", got)
	}
}

func TestInitConfigReadsEnvironment(t *testing.T) {
	t.Setenv("APP_PORT", "9000")
	t.Setenv("DB_HOST", "db")
	t.Setenv("DB_PORT", "6543")
	t.Setenv("DB_NAME", "other")

	cfg, err := config.InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}

	if cfg.AppPort != "9000" {
		t.Errorf("AppPort = %q, want 9000", cfg.AppPort)
	}

	if cfg.Postgres.Host != "db" || cfg.Postgres.Port != 6543 || cfg.Postgres.Name != "other" {
		t.Errorf("Postgres = %+v", cfg.Postgres)
	}
}

// A non-numeric port must fail at startup rather than silently becoming 0 and
// producing a DSN that fails to connect for a much less obvious reason.
func TestInitConfigRejectsNonNumericPort(t *testing.T) {
	t.Setenv("DB_PORT", "not-a-port")

	if _, err := config.InitConfig(); err == nil {
		t.Fatal("InitConfig() error = nil, want failure")
	}
}
