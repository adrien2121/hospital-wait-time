package sites

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/adrien2121/GoProject/internal/clock"
	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/scraper"
)

const (
	// MontfortSourceURL is the production upstream. Exported so production wiring
	// and integration tests name the same URL (tests substitute an httptest.Server
	// URL via the constructor).
	MontfortSourceURL = "https://hopitalmontfort.com/fr/urgence"
	montfortInterval  = 15 * time.Minute
)

type montfortScraper struct {
	scraper.BaseScraper
	client    HTTPGetter
	sourceURL string
	clock     clock.Clock
}

var _ scraper.Scraper = (*montfortScraper)(nil)

// NewMontfortScraper wires the Montfort scraper. sourceURL and clock are injected
// for the same reasons documented on NewCHEOScraper.
func NewMontfortScraper(client HTTPGetter, sourceURL string, c clock.Clock) scraper.Scraper {
	return &montfortScraper{
		BaseScraper: scraper.NewBase(domain.HospitalIDMontfort, montfortInterval),
		client:      client,
		sourceURL:   sourceURL,
		clock:       c,
	}
}

func (s *montfortScraper) Scrape(ctx context.Context) ([]domain.WaitTimeSnapshot, error) {
	body, err := s.client.Get(ctx, s.sourceURL)
	if err != nil {
		return nil, fmt.Errorf("montfortScraper.Scrape: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("montfortScraper.Scrape parse html: %w", err)
	}

	// First .gros-chiffres inside the wait block is the median wait in minutes.
	waitText := strings.TrimSpace(
		doc.Find(".block-content-block_nouveau_temps_d_attente .gros-chiffres").First().Text(),
	)

	// "--" means temporarily unavailable; treat as transient, not an error.
	if waitText == "--" || waitText == "" {
		return nil, fmt.Errorf("montfortScraper.Scrape: wait time temporarily unavailable")
	}

	minutes, err := scraper.ParseWaitTime(waitText)
	if err != nil {
		return nil, fmt.Errorf("montfortScraper.Scrape parse wait time: %w", err)
	}

	now := s.clock.Now()
	return []domain.WaitTimeSnapshot{{
		HospitalID:  s.HospitalID(),
		WaitMinutes: minutes,
		Category:    domain.WaitCategoryTriageToDoctor,
		RecordedAt:  now,
		ScrapedAt:   now,
	}}, nil
}
