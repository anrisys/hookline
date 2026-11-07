// Package config loads process configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Config is the configuration for every mode; each mode reads what it needs.
type Config struct {
	DatabaseURL     string
	APIAddr         string
	WorkerAddr      string
	ChaosAddr       string
	ShutdownTimeout time.Duration
	LogLevel        string
}

// Load reads configuration for mode ("api", "worker" or "chaos") using getenv.
// Empty values count as unset. It applies defaults and returns every
// validation error at once.
func Load(mode string, getenv func(string) string) (Config, error) {
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		DatabaseURL: get("DATABASE_URL", ""),
		APIAddr:     get("API_ADDR", ":8080"),
		WorkerAddr:  get("WORKER_ADDR", ":8081"),
		ChaosAddr:   get("CHAOS_ADDR", ":8090"),
		LogLevel:    get("LOG_LEVEL", "info"),
	}

	var errs []error
	switch mode {
	case "api", "worker":
		if cfg.DatabaseURL == "" {
			errs = append(errs, errors.New("DATABASE_URL is required"))
		}
	case "chaos":
	default:
		errs = append(errs, fmt.Errorf("unknown mode %q", mode))
	}

	raw := get("SHUTDOWN_TIMEOUT", "30s")
	d, err := time.ParseDuration(raw)
	switch {
	case err != nil:
		errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT: invalid duration %q", raw))
	case d <= 0:
		errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT: must be positive, got %q", raw))
	}
	cfg.ShutdownTimeout = d

	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: want debug, info, warn or error, got %q", cfg.LogLevel))
	}

	return cfg, errors.Join(errs...)
}
