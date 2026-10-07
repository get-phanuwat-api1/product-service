package config

import "os"

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	HTTPPort    string
	DatabaseURL string
}

// Load reads configuration from the environment with sensible local defaults.
func Load() Config {
	return Config{
		HTTPPort:    getEnv("HTTP_PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/product?sslmode=disable"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
