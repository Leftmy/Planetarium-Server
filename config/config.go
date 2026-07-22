package config

import (
	"github.com/leftmy/planetarium-server/internal/adapter/postgres"
)

// Config - this is the general config of application
type Config struct {
	AppPort  string          `env:"APP_PORT"`
	Postgres postgres.Config // Import adapter's config
}

func InitConfig() (Config, error) {
	// there will be parsing and validation of config
	return Config{}, nil
}
