package config

import (
	"strings"
	"testing"
	"time"
)

// Validate is called inside Load and is safe to call from a binary's bootstrap step.
// It enforces the runtime constraints we cannot let the binaries start without (e.g. a zero
// rate limit would let the scraper hammer hospital sites with no delay).

func validConfig() Config {
	return Config{
		DatabaseURL:           "postgres://localhost/test",
		APIAddr:               ":8080",
		ScrapeInterval:        15 * time.Minute,
		LogLevel:              "info",
		RateLimitPerDomainSec: 30,
	}
}

func TestValidate_Accepts(t *testing.T) {
	t.Run(`
		given a fully populated config,
		when Validate runs,
		then it accepts the config and returns nil`,
		func(t *testing.T) {
			// Given: a fully-valid config.
			c := validConfig()

			// When: the binary's bootstrap step asks Validate to gate startup.
			err := c.Validate()

			// Then: Validate returns nil and the binary boots.
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		},
	)
}

func TestValidate_Rejects(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(c *Config)
		wantErrs []string
	}{
		{
			name: `
				given a config with DATABASE_URL unset,
				when Validate runs,
				then the error names DATABASE_URL`,
			mutate:   func(c *Config) { c.DatabaseURL = "" },
			wantErrs: []string{"DATABASE_URL"},
		},
		{
			name: `
				given a config where API_PORT was unset (APIAddr is just ':'),
				when Validate runs,
				then the error names API_PORT`,
			mutate:   func(c *Config) { c.APIAddr = ":" },
			wantErrs: []string{"API_PORT"},
		},
		{
			name: `
				given a config where RateLimitPerDomainSec is 0
				(which would disable polite rate limiting against hospital sites),
				when Validate runs,
				then the error names RATE_LIMIT`,
			mutate:   func(c *Config) { c.RateLimitPerDomainSec = 0 },
			wantErrs: []string{"RATE_LIMIT"},
		},
		{
			name: `
				given a config where RateLimitPerDomainSec is negative,
				when Validate runs,
				then the error names RATE_LIMIT`,
			mutate:   func(c *Config) { c.RateLimitPerDomainSec = -1 },
			wantErrs: []string{"RATE_LIMIT"},
		},
		{
			name: `
				given a config where ScrapeInterval is zero (would tight-loop the orchestrator),
				when Validate runs,
				then the error names SCRAPE_INTERVAL`,
			mutate:   func(c *Config) { c.ScrapeInterval = 0 },
			wantErrs: []string{"SCRAPE_INTERVAL"},
		},
		{
			name: `
				given a config with multiple bad fields at once,
				when Validate runs,
				then the joined error names every bad field in one message`,
			mutate: func(c *Config) {
				c.DatabaseURL = ""
				c.RateLimitPerDomainSec = 0
			},
			wantErrs: []string{"DATABASE_URL", "RATE_LIMIT"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given: start from a fully-valid config, then apply the row's mutation.
			c := validConfig()
			tc.mutate(&c)

			// When: the binary's bootstrap step asks Validate to gate startup.
			err := c.Validate()

			// Then: Validate returns a non-nil joined error mentioning every named field.
			if err == nil {
				t.Fatalf("expected error containing %v, got nil", tc.wantErrs)
			}
			for _, want := range tc.wantErrs {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q missing substring %q", err.Error(), want)
				}
			}
		})
	}
}

func TestLoad_DefaultsWhenOnlyDATABASEURLIsSet(t *testing.T) {
	t.Run(`
		given only DATABASE_URL is set in the environment,
		when Load runs,
		then every other config field is populated from its hard-coded default`,
		func(t *testing.T) {
			// Given: only DATABASE_URL set, all other env vars cleared.
			t.Setenv("DATABASE_URL", "postgres://localhost/test")
			t.Setenv("API_PORT", "")
			t.Setenv("SCRAPE_INTERVAL_SECONDS", "")
			t.Setenv("RATE_LIMIT_PER_DOMAIN_SECONDS", "")
			t.Setenv("LOG_LEVEL", "")

			// When: the binary calls Load at startup.
			cfg, err := Load()

			// Then: Load succeeds and every defaultable field falls back to its hard-coded default.
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.APIAddr != ":8080" {
				t.Errorf("APIAddr = %q, want :8080", cfg.APIAddr)
			}
			if cfg.ScrapeInterval != 15*time.Minute {
				t.Errorf("ScrapeInterval = %v, want 15m", cfg.ScrapeInterval)
			}
			if cfg.RateLimitPerDomainSec != 30 {
				t.Errorf("RateLimitPerDomainSec = %d, want 30", cfg.RateLimitPerDomainSec)
			}
			if cfg.LogLevel != "info" {
				t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
			}
		},
	)
}

func TestLoad_RejectsNonIntegerInterval(t *testing.T) {
	t.Run(`
		given SCRAPE_INTERVAL_SECONDS is set to a non-integer string,
		when Load runs,
		then it returns an error and the binary fails fast at boot`,
		func(t *testing.T) {
			// Given: SCRAPE_INTERVAL_SECONDS contains junk text.
			t.Setenv("DATABASE_URL", "postgres://localhost/test")
			t.Setenv("SCRAPE_INTERVAL_SECONDS", "not-a-number")

			// When: the binary calls Load at startup.
			_, err := Load()

			// Then: Load returns an error.
			if err == nil {
				t.Fatal("expected error for non-integer SCRAPE_INTERVAL_SECONDS")
			}
		},
	)
}
