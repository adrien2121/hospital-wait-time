package scraper

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/adrien2121/GoProject/internal/repository"
)

const (
	maxBackoff       = 1 * time.Hour
	backoffThreshold = 3 // start doubling interval after this many consecutive failures
	scrapeTimeout    = 30 * time.Second
)

type Orchestrator struct {
	scrapers []Scraper
	repo     repository.WaitTimeRepository
	log      *slog.Logger
}

func NewOrchestrator(scrapers []Scraper, repo repository.WaitTimeRepository, log *slog.Logger) *Orchestrator {
	return &Orchestrator{scrapers: scrapers, repo: repo, log: log}
}

// Run starts one goroutine per scraper and blocks until ctx is cancelled.
// ctx comes from cmd/scraper/main.go via signal.NotifyContext — SIGINT/SIGTERM cancels it.
func (o *Orchestrator) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, s := range o.scrapers {
		wg.Add(1)
		go func(s Scraper) {
			defer wg.Done()
			o.runScraper(ctx, s)
		}(s)
	}
	wg.Wait() // blocks until all goroutines exit (ctx cancelled → all return)
}

func (o *Orchestrator) runScraper(ctx context.Context, s Scraper) {
	// Stagger first run so all scrapers don't hit their sites simultaneously.
	jitter := time.Duration(rand.N(int64(30 * time.Second))) //nolint:gosec // G404: stagger jitter, not crypto
	select {
	case <-ctx.Done(): // shutdown before first scrape; exit immediately
		return
	case <-time.After(jitter):
	}

	consecutiveFails := 0
	consecutiveSaveFails := 0

	for {
		scrapeCtx, cancel := context.WithTimeout(ctx, scrapeTimeout)
		snapshots, err := s.Scrape(scrapeCtx)
		cancel() // release the WithTimeout timer now that Scrape returned; not calling it leaks a goroutine per loop

		if err != nil {
			consecutiveFails++
			o.log.Error("scrape failed", "hospital", s.HospitalID(), "err", err, "consecutive_fails", consecutiveFails)
		} else {
			consecutiveFails = 0 // reset: backoff only tracks consecutive failures; one success = back to normal
			for _, snap := range snapshots {
				if saveErr := o.repo.Save(ctx, snap); saveErr != nil {
					consecutiveSaveFails++
					o.log.Error("save failed", "hospital", s.HospitalID(), "err", saveErr, "consecutive_save_fails", consecutiveSaveFails)
				} else {
					consecutiveSaveFails = 0
				}
			}
		}

		next := nextInterval(s.Interval(), consecutiveFails)
		o.log.Debug("scraper sleeping", "hospital", s.HospitalID(), "next_in", next)

		select {
		case <-ctx.Done(): // SIGINT/SIGTERM received; exit cleanly instead of sleeping
			return
		case <-time.After(next):
		}
	}
}

// nextInterval returns the wait duration before the next scrape.
// Normal: use scraper's configured interval (e.g. 15m).
// After backoffThreshold consecutive failures: double each time, capped at maxBackoff.
// Doubles step-by-step with a min() cap each step so the multiplier can never overflow.
// Package-level fn (no receiver) so it can be table-tested without an Orchestrator.
func nextInterval(base time.Duration, fails int) time.Duration {
	if fails < backoffThreshold {
		return base
	}
	next := base
	for range fails - backoffThreshold + 1 {
		next = min(next*2, maxBackoff)
	}
	return next
}
