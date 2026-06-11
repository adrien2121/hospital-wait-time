package sites

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/adrien2121/GoProject/internal/clock"
	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/httpclient"
	"github.com/adrien2121/GoProject/internal/repository"
	"github.com/adrien2121/GoProject/internal/runner"
	"github.com/adrien2121/GoProject/internal/scraper"
)

const (
	// MontfortSourceURL is the production upstream. Exported so production wiring
	// and integration tests name the same URL (tests substitute an httptest.Server
	// URL via the constructor).
	MontfortSourceURL = "https://hopitalmontfort.com/fr/urgence"
	montfortInterval  = 15 * time.Minute
)

type MontfortScraper struct {
	scraper.BaseScraper
	client    httpclient.Getter
	sourceURL string
	clock     clock.Clock
}

// Compile-time check: MontfortScraper must satisfy runner.Runnable.
var _ runner.Runnable = (*MontfortScraper)(nil)

// NewMontfortScraper wires the Montfort scraper. sourceURL and clock are injected
// for the same reasons documented on NewCHEOScraper. repo and log flow through
// to BaseScraper so Run can save snapshots without the scheduler holding the
// repository.
func NewMontfortScraper(client httpclient.Getter, sourceURL string, c clock.Clock, repo repository.WaitTimeRepository, log *slog.Logger) *MontfortScraper {
	return &MontfortScraper{
		BaseScraper: scraper.NewBase(domain.HospitalIDMontfort, montfortInterval, repo, log),
		client:      client,
		sourceURL:   sourceURL,
		clock:       c,
	}
}

// Run fetches the Montfort emergency page, extracts the published median wait,
// and saves a snapshot. The "--" branch returns ErrTransientUnavailable, which
// wraps runner.ErrSkip so the scheduler does not bump the backoff counter for
// upstream-reported "no data right now" states.
func (s *MontfortScraper) Run(ctx context.Context) error {
	body, err := s.client.Get(ctx, s.sourceURL)
	if err != nil {
		return fmt.Errorf("montfortScraper.Run: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("montfortScraper.Run parse html: %w", err)
	}

	// First .gros-chiffres inside the wait block is the median wait in minutes.
	waitText := strings.TrimSpace(
		doc.Find(".block-content-block_nouveau_temps_d_attente .gros-chiffres").First().Text(),
	)

	// "--" means temporarily unavailable. Wrap the ErrTransientUnavailable
	// sentinel with %w so the scheduler can recognize it via errors.Is
	// (transitively matching runner.ErrSkip) and skip backoff.
	if waitText == "--" || waitText == "" {
		s.Logger().Warn("scrape transient unavailable", "hospital", s.HospitalID())
		return fmt.Errorf("montfortScraper.Run: %w", scraper.ErrTransientUnavailable)
	}

	minutes, err := scraper.ParseWaitTime(waitText)
	if err != nil {
		return fmt.Errorf("montfortScraper.Run parse wait time: %w", err)
	}

	now := s.clock.Now()
	snaps := []domain.WaitTimeSnapshot{{
		HospitalID:  s.HospitalID(),
		WaitMinutes: minutes,
		Category:    domain.WaitCategoryTriageToDoctor,
		RecordedAt:  now,
		ScrapedAt:   now,
	}}
	s.SaveSnapshots(ctx, snaps)
	return nil
}
