package sites

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/adrien2121/GoProject/internal/clock"
	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/scraper"
)

const (
	// CHEOAPIURL is the production upstream. Exported so production wiring and
	// integration tests both name it (tests substitute an httptest.Server URL via
	// the constructor; the constant stays the source of truth for prod).
	CHEOAPIURL   = "https://www.cheo.on.ca/Common/Services/GetWaitTimesV2.ashx?&lang=en"
	cheoInterval = 15 * time.Minute
)

type cheoScraper struct {
	scraper.BaseScraper
	client HTTPGetter
	apiURL string
	clock  clock.Clock
}

var _ scraper.Scraper = (*cheoScraper)(nil)

// NewCHEOScraper wires the CHEO scraper. apiURL is injected so tests can point at
// an httptest.Server; clock is injected so tests can control the timestamp on
// every emitted snapshot (needed for the trend window and anomaly baseline).
func NewCHEOScraper(client HTTPGetter, apiURL string, c clock.Clock) scraper.Scraper {
	return &cheoScraper{
		BaseScraper: scraper.NewBase(domain.HospitalIDCHEO, cheoInterval),
		client:      client,
		apiURL:      apiURL,
		clock:       c,
	}
}

func (s *cheoScraper) Scrape(ctx context.Context) ([]domain.WaitTimeSnapshot, error) {
	body, err := s.client.Get(ctx, s.apiURL)
	if err != nil {
		return nil, fmt.Errorf("cheoScraper.Scrape: %w", err)
	}

	var resp struct {
		LongestWaitMin float64 `json:"longestWaitMin"`
		AveWaitMin     float64 `json:"aveWaitMin"`
		PatientCount   int     `json:"patientCount"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("cheoScraper.Scrape parse json: %w", err)
	}
	if resp.LongestWaitMin == 0 {
		return nil, fmt.Errorf("cheoScraper.Scrape: longestWaitMin is 0 or missing")
	}

	now := s.clock.Now()
	return []domain.WaitTimeSnapshot{{
		HospitalID:  s.HospitalID(),
		WaitMinutes: int(resp.LongestWaitMin),
		Category:    domain.WaitCategoryTriageToDoctor,
		RecordedAt:  now,
		ScrapedAt:   now,
	}}, nil
}
