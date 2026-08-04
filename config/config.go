package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/leftmy/planetarium-server/internal/adapter/postgres"
)

// DevJWTSecret is the fallback signing key. It exists so that a fresh checkout
// runs without setup, and it is worthless as a secret precisely because it is
// published here.
//
// Whoever builds the token manager is expected to notice it and complain — see
// cmd/api. The warning does not live here because reading configuration and
// using it are different jobs, and the migrate command reads the same config
// without touching tokens at all.
const DevJWTSecret = "insecure-development-secret-change-me"

// Auth holds the settings behind session tokens.
type Auth struct {
	JWTSecret  string        `env:"JWT_SECRET"`
	AccessTTL  time.Duration `env:"ACCESS_TTL"`
	RefreshTTL time.Duration `env:"REFRESH_TTL"`
}

// Config - this is the general config of application
type Config struct {
	AppPort  string          `env:"APP_PORT"`
	Postgres postgres.Config // Import adapter's config
	Auth     Auth
}

// InitConfig reads the variables documented in .env.example. The defaults match
// that file so a fresh checkout runs against docker compose without setup, while
// a deployment overrides them through the environment.
func InitConfig() (Config, error) {
	port, err := strconv.ParseUint(env("DB_PORT", "5432"), 10, 16)
	if err != nil {
		return Config{}, fmt.Errorf("DB_PORT: %w", err)
	}

	accessTTL, err := time.ParseDuration(env("ACCESS_TTL", "15m"))
	if err != nil {
		return Config{}, fmt.Errorf("ACCESS_TTL: %w", err)
	}

	refreshTTL, err := time.ParseDuration(env("REFRESH_TTL", "720h"))
	if err != nil {
		return Config{}, fmt.Errorf("REFRESH_TTL: %w", err)
	}

	return Config{
		AppPort: env("APP_PORT", "8080"),
		Postgres: postgres.Config{
			User:     env("DB_USER", "postgres"),
			Password: env("DB_PASSWORD", "secret"),
			Host:     env("DB_HOST", "localhost"),
			Port:     uint(port),
			Name:     env("DB_NAME", "planetarium_db"),
		},
		Auth: Auth{
			JWTSecret:  env("JWT_SECRET", DevJWTSecret),
			AccessTTL:  accessTTL,
			RefreshTTL: refreshTTL,
		},
	}, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
