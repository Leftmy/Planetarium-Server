package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/leftmy/planetarium-server/internal/adapter/postgres"
)

// Config - this is the general config of application
type Config struct {
	AppPort  string          `env:"APP_PORT"`
	Postgres postgres.Config // Import adapter's config
}

// InitConfig reads the variables documented in .env.example. The defaults match
// that file so a fresh checkout runs against docker compose without setup, while
// a deployment overrides them through the environment.
func InitConfig() (Config, error) {
	port, err := strconv.ParseUint(env("DB_PORT", "5432"), 10, 16)
	if err != nil {
		return Config{}, fmt.Errorf("DB_PORT: %w", err)
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
	}, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
