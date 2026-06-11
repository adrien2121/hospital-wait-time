package main

import (
	"log/slog"

	"github.com/adrien2121/GoProject/internal/clock"
	"github.com/adrien2121/GoProject/internal/collector"
	"github.com/adrien2121/GoProject/internal/httpclient"
	"github.com/adrien2121/GoProject/internal/logger"
	"github.com/adrien2121/GoProject/internal/repository"
	"github.com/adrien2121/GoProject/internal/runner"
	"github.com/adrien2121/GoProject/internal/scraper/sites"
)

// buildRunners returns every Runnable the scraper binary drives. All
// dependencies are injected so main owns their lifecycle. The shared HTTP
// client is fine across scrapers and collectors because the per-hostname rate
// limiter buckets keep each source isolated.
func buildRunners(
	client httpclient.Getter,
	c clock.Clock,
	waitTimes repository.WaitTimeRepository,
	signals repository.ExternalSignalRepository,
	log *slog.Logger,
) []runner.Runnable {
	return []runner.Runnable{
		sites.NewCHEOScraper(client, sites.CHEOAPIURL, c, waitTimes, log),
		sites.NewMontfortScraper(client, sites.MontfortSourceURL, c, waitTimes, log),
		collector.NewWeatherCollector(client, collector.SWOBURL, c, signals, log),
		collector.NewAQHICollector(client, collector.AQHIURL, c, signals, log),
	}
}

func buildLogger(logLevel string) *slog.Logger { return logger.Build(logLevel) }
