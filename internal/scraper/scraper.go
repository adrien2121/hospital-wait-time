package scraper

import (
	"context"
	"time"

	"github.com/adrien2121/GoProject/internal/domain"
)

// Scraper fetches wait time data from a single hospital source.
// Each hospital has its own implementation of this interface.
type Scraper interface {
	// HospitalID is the stable ID matching the hospitals table.
	HospitalID() string
	// Scrape fetches current wait times for this hospital.
	Scrape(ctx context.Context) ([]domain.WaitTimeSnapshot, error)
	// Interval is how often the orchestrator should run this scraper.
	Interval() time.Duration
}

// BaseScraper provides the shared HospitalID/Interval boilerplate for site
// implementations. Site scrapers embed this and only implement Scrape().
type BaseScraper struct {
	id    string
	every time.Duration
}

// NewBase returns a BaseScraper carrying the given hospital ID and scrape interval.
func NewBase(hospitalID string, interval time.Duration) BaseScraper {
	return BaseScraper{id: hospitalID, every: interval}
}

func (b BaseScraper) HospitalID() string      { return b.id }
func (b BaseScraper) Interval() time.Duration { return b.every }
