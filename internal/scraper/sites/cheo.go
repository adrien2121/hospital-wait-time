package sites

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/adrien2121/GoProject/internal/clock"
	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/httpclient"
	"github.com/adrien2121/GoProject/internal/repository"
	"github.com/adrien2121/GoProject/internal/runner"
	"github.com/adrien2121/GoProject/internal/scraper"
)

const (
	// CHEOAPIURL is the production upstream. Exported so production wiring and
	// integration tests both name it (tests substitute an httptest.Server URL via
	// the constructor; the constant stays the source of truth for prod).
	CHEOAPIURL   = "https://www.cheo.on.ca/Common/Services/GetWaitTimesV2.ashx?&lang=en"
	cheoInterval = 15 * time.Minute
)

type CHEOScraper struct {
	scraper.BaseScraper
	client httpclient.Getter
	apiURL string
	clock  clock.Clock
}

// Compile-time check: CHEOScraper must satisfy runner.Runnable.
var _ runner.Runnable = (*CHEOScraper)(nil)

// NewCHEOScraper wires the CHEO scraper. apiURL is injected so tests can point at
// an httptest.Server; clock is injected so tests can control the timestamp on
// every emitted snapshot (needed for the trend window and anomaly baseline).
// repo and log are passed through to BaseScraper so each Run can save snapshots
// without the scheduler holding the repository.
func NewCHEOScraper(client httpclient.Getter, apiURL string, c clock.Clock, repo repository.WaitTimeRepository, log *slog.Logger) *CHEOScraper {
	return &CHEOScraper{
		BaseScraper: scraper.NewBase(domain.HospitalIDCHEO, cheoInterval, repo, log),
		client:      client,
		apiURL:      apiURL,
		clock:       c,
	}
}

// Run fetches the current CHEO wait-time payload, builds a snapshot, and
// hands it to the BaseScraper save helper. Returning an error feeds the
// scheduler's backoff counter; returning nil resets it.
func (s *CHEOScraper) Run(ctx context.Context) error {
	body, err := s.client.Get(ctx, s.apiURL)
	if err != nil {
		return fmt.Errorf("cheoScraper.Run: %w", err)
	}

	var resp struct {
		LongestWaitMin float64 `json:"longestWaitMin"`
		AveWaitMin     float64 `json:"aveWaitMin"`
		PatientCount   int     `json:"patientCount"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("cheoScraper.Run parse json: %w", err)
	}
	// aveWaitMin represents expected triage-to-doctor time for a new arrival.
	// longestWaitMin is the worst patient currently waiting and is misleading
	// as a user-facing estimate, so we ignore it for now.
	if resp.AveWaitMin == 0 {
		return fmt.Errorf("cheoScraper.Run: aveWaitMin is 0 or missing")
	}

	now := s.clock.Now()
	snaps := []domain.WaitTimeSnapshot{{
		HospitalID:  s.HospitalID(),
		WaitMinutes: int(resp.AveWaitMin),
		Category:    domain.WaitCategoryTriageToDoctor,
		RecordedAt:  now,
		ScrapedAt:   now,
	}}
	s.SaveSnapshots(ctx, snaps)
	return nil
}
