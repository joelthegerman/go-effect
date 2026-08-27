// Package config loads runtime configuration from the environment with
// sensible, dev-friendly defaults so `make run` works against the sandbox
// docker-compose Postgres out of the box.
package config

import (
	"os"
	"time"
)

type Config struct {
	Addr            string
	DatabaseURL     string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// Load reads configuration from the environment. Only DATABASE_URL and PORT are
// commonly overridden; the timeouts have production-reasonable defaults.
func Load() Config {
	return Config{
		Addr:            ":" + env("PORT", "8080"),
		DatabaseURL:     env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/sandbox?sslmode=disable"),
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 15 * time.Second,
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
