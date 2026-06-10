package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration sourced from environment variables.
type Config struct {
	DatabaseURL           string
	APIAddr               string
	ScrapeInterval        time.Duration
	LogLevel              string
	RateLimitPerDomainSec int
}

// Load reads configuration from environment variables.
// All required variables must be set; optional ones fall back to defaults.
// Returns a joined error listing every invalid field at once.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		APIAddr:               ":" + envOr("API_PORT", "8080"),
		LogLevel:              envOr("LOG_LEVEL", "info"),
		ScrapeInterval:        15 * time.Minute,
		RateLimitPerDomainSec: 30,
	}

	if raw := os.Getenv("SCRAPE_INTERVAL_SECONDS"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("SCRAPE_INTERVAL_SECONDS must be an integer: %w", err)
		}
		cfg.ScrapeInterval = time.Duration(v) * time.Second
	}
	if raw := os.Getenv("RATE_LIMIT_PER_DOMAIN_SECONDS"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("RATE_LIMIT_PER_DOMAIN_SECONDS must be an integer: %w", err)
		}
		cfg.RateLimitPerDomainSec = v
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate enforces semantic constraints on parsed config.
// Errors are joined so an operator sees every misconfig at once instead of fixing them one by one.
// Caught here: empty DATABASE_URL, RateLimitPerDomainSec=0 (would let the limiter burst against hospital sites),
// ScrapeInterval=0 (would loop the orchestrator with no delay).
func (c Config) Validate() error {
	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if c.APIAddr == ":" {
		errs = append(errs, errors.New("API_PORT is required"))
	}
	if c.ScrapeInterval <= 0 {
		errs = append(errs, errors.New("SCRAPE_INTERVAL_SECONDS must be > 0"))
	}
	if c.RateLimitPerDomainSec <= 0 {
		errs = append(errs, errors.New("RATE_LIMIT_PER_DOMAIN_SECONDS must be > 0 (zero would disable polite rate limiting)"))
	}
	return errors.Join(errs...)
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
