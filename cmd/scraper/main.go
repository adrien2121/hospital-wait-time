package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/adrien2121/GoProject/internal/clock"
	"github.com/adrien2121/GoProject/internal/collector"
	"github.com/adrien2121/GoProject/internal/config"
	"github.com/adrien2121/GoProject/internal/httpclient"
	"github.com/adrien2121/GoProject/internal/scraper"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := buildLogger(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := openScraperStorage(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	scrapers := buildScrapers(cfg.RateLimitPerDomainSec)
	orch := scraper.NewOrchestrator(scrapers, store.waitTimeRepo, logger)

	// Separate client so collectors don't contend with hospital scrapers.
	httpClient := httpclient.New(cfg.RateLimitPerDomainSec)
	clk := clock.RealClock{}
	weather := collector.NewWeatherCollector(httpClient, collector.SWOBURL, clk, store.externalSignalRepo, logger)
	aqhi := collector.NewAQHICollector(httpClient, collector.AQHIURL, clk, store.externalSignalRepo, logger)

	logger.Info("scraper starting", "hospitals", len(scrapers))

	// errgroup: any component returning an error (or panicking) cancels the others.
	// Clean shutdown (ctx cancelled via SIGINT/SIGTERM) propagates as context.Canceled, which is not a real failure.
	g, gctx := errgroup.WithContext(ctx)
	g.Go(supervise(gctx, "weather", logger, weather.Run))
	g.Go(supervise(gctx, "aqhi", logger, aqhi.Run))
	g.Go(supervise(gctx, "orchestrator", logger, orch.Run))

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("component failed: %w", err)
	}
	logger.Info("scraper stopped")
	return nil
}

// supervise wraps a long-running Run(ctx) so panics surface as group errors.
// Without this, a panic in one collector would crash the whole process with no structured log.
// Returning nil on normal exit lets ctx cancellation propagate as the group's first error.
func supervise(ctx context.Context, name string, logger *slog.Logger, run func(context.Context)) func() error {
	return func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("component panic", "component", name, "panic", r, "stack", string(debug.Stack()))
				err = fmt.Errorf("%s panic: %v", name, r)
			}
		}()
		run(ctx)
		return ctx.Err() // returns context.Canceled on shutdown, nil if Run exited cleanly first
	}
}
