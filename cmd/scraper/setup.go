package main

import (
	"log/slog"

	"github.com/adrien2121/GoProject/internal/clock"
	"github.com/adrien2121/GoProject/internal/httpclient"
	"github.com/adrien2121/GoProject/internal/logger"
	"github.com/adrien2121/GoProject/internal/scraper"
	"github.com/adrien2121/GoProject/internal/scraper/sites"
)

// buildScrapers creates one scraper per hospital using a shared polite HTTP client.
// The client enforces per-domain rate limiting so no hospital site gets hammered.
// TOH and QCH do not publish live wait times; add their scrapers here when feeds exist.
func buildScrapers(rateLimitPerDomainSec int) []scraper.Scraper {
	client := httpclient.New(rateLimitPerDomainSec)
	c := clock.RealClock{}
	return []scraper.Scraper{
		sites.NewCHEOScraper(client, sites.CHEOAPIURL, c),
		sites.NewMontfortScraper(client, sites.MontfortSourceURL, c),
	}
}

func buildLogger(logLevel string) *slog.Logger { return logger.Build(logLevel) }
