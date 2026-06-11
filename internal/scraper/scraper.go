// Package scraper holds wait-time-specific helpers shared by every hospital
// scraper site impl (in subpackage sites). The scheduler that drives those
// scrapers lives in internal/runner; this pkg only owns the pieces that are
// specific to wait-time scraping: the transient-state sentinel and a base
// struct that exposes the snapshot save helper.
package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/repository"
	"github.com/adrien2121/GoProject/internal/runner"
)

// ErrTransientUnavailable is a sentinel error: a fixed exported value callers
// match by identity instead of parsing message strings (like io.EOF). Site
// scrapers return it wrapped with %w when the upstream is reachable but
// reports "no data right now" (e.g. Montfort renders "--"). It wraps
// runner.ErrSkip so the scheduler recognizes the transient state and skips its
// consecutive-failure backoff without any scraper-pkg-specific handling.
var ErrTransientUnavailable = fmt.Errorf("source temporarily unavailable: %w", runner.ErrSkip)

// BaseScraper carries the per-site state common to every wait-time scraper:
// the hospital identity (via embedded runner.Base), the repository it saves
// snapshots to, the structured logger, and the rolling save-failure counter
// used only for logging.
//
// Site impls embed BaseScraper, satisfy runner.Runnable via the embedded
// runner.Base plus their own Run method, and call SaveSnapshots to persist
// the snapshots they fetch. A separate scrape-failure counter is owned by
// the runner itself and feeds backoff.
type BaseScraper struct {
	runner.Base
	repo repository.WaitTimeRepository
	log  *slog.Logger

	// consecutiveSaveFails is single-writer: only the goroutine running this
	// site's Run mutates it, because runner.Run gives each Runnable its own
	// goroutine. Save failures log but do not drive backoff, since DB issues
	// are orthogonal to upstream health and would only delay catching real
	// upstream recovery.
	consecutiveSaveFails int
}

// NewBase wires the per-site state every wait-time scraper needs. Pass the
// hospital ID, the scrape cadence, the repository the site will save into,
// and the structured logger.
func NewBase(hospitalID string, interval time.Duration, repo repository.WaitTimeRepository, log *slog.Logger) BaseScraper {
	return BaseScraper{
		Base: runner.NewBase(hospitalID, interval),
		repo: repo,
		log:  log,
	}
}

// HospitalID returns the hospital identity. Kept as an explicit accessor on
// top of the embedded runner.Base.Name() because site code reads better when
// the verb names the domain concept.
func (b *BaseScraper) HospitalID() string { return b.Name() }

// Logger exposes the structured logger so site impls in subpackages can log
// site-specific events (e.g. Montfort's transient-unavailable Warn). Returns
// the same *slog.Logger held in the unexported field; no allocation.
func (b *BaseScraper) Logger() *slog.Logger { return b.log }

// SaveSnapshots persists each snapshot returned by a single Run pass and logs
// the outcome with per-hospital context. Save failures log but do not
// propagate, since DB issues should not drive scrape backoff. Sites that need
// different save semantics (per-snapshot dedupe, batch insert, raw-payload
// audit) can skip this helper and roll their own.
func (b *BaseScraper) SaveSnapshots(ctx context.Context, snaps []domain.WaitTimeSnapshot) {
	for _, snap := range snaps {
		if err := b.repo.Save(ctx, snap); err != nil {
			b.consecutiveSaveFails++
			b.log.Error("save failed", "hospital", b.Name(), "err", err, "consecutive_save_fails", b.consecutiveSaveFails)
			continue
		}
		b.consecutiveSaveFails = 0
		b.log.Info("scrape ok",
			"hospital", b.Name(),
			"wait_minutes", snap.WaitMinutes,
			"category", snap.Category,
			"recorded_at", snap.RecordedAt.Format(time.RFC3339),
		)
	}
}
