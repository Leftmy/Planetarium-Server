package postgres

import "fmt"

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

func New(cfg Config) error {
	_ = cfg.BuildURL() // There will be connection to pgxpool
	return nil
}
