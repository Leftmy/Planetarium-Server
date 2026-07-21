package config

// Config holds all application configurations (DB, API keys, etc.)
type Config struct {
	Port        string
	DatabaseURL string
}

// Load reads environment variables and populates the Config struct
func Load() (*Config, error) {
	// Logic to load .env or environment variables will go here
	return &Config{}, nil
}
