package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/adrien2121/GoProject/internal/clock"
	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/repository"
)

const (
	// AQHIURL is the production upstream. Exported so production wiring and
	// integration tests name the same URL (tests substitute an httptest.Server URL
	// via the constructor).
	// Ottawa bounding box; latest=true returns the single most recent AQHI reading.
	AQHIURL      = "https://api.weather.gc.ca/collections/aqhi-observations-realtime/items?f=json&limit=1&bbox=-75.9,45.2,-75.4,45.5&latest=true"
	aqhiInterval = 1 * time.Hour
	aqhiTimeout  = 30 * time.Second
)

// AQHICollector polls Environment Canada's real-time AQHI endpoint.
// AQHI (Air Quality Health Index) updates hourly on a 1–10+ scale.
type AQHICollector struct {
	client HTTPGetter
	apiURL string
	clock  clock.Clock
	repo   repository.ExternalSignalRepository
	log    *slog.Logger
}

// NewAQHICollector wires the AQHI collector.
func NewAQHICollector(client HTTPGetter, apiURL string, c clock.Clock, repo repository.ExternalSignalRepository, log *slog.Logger) *AQHICollector {
	return &AQHICollector{client: client, apiURL: apiURL, clock: c, repo: repo, log: log}
}

// Run loops forever collecting AQHI every aqhiInterval.
func (a *AQHICollector) Run(ctx context.Context) {
	for {
		collectCtx, cancel := context.WithTimeout(ctx, aqhiTimeout)
		if err := a.Collect(collectCtx); err != nil {
			a.log.Error("aqhi collect failed", "err", err)
		}
		cancel() // release the WithTimeout timer now that collect returned; not calling it leaks a goroutine per loop
		select {
		case <-ctx.Done():
			return
		case <-time.After(aqhiInterval):
		}
	}
}

type aqhiResponse struct {
	Features []struct {
		Properties struct {
			AQHI                float64 `json:"aqhi"`
			ObservationDatetime string  `json:"observation_datetime"`
		} `json:"properties"`
	} `json:"features"`
}

// Collect performs one fetch+save pass.
func (a *AQHICollector) Collect(ctx context.Context) error {
	aqhi, observedAt, raw, err := a.fetch(ctx)
	if err != nil {
		return err
	}
	return a.save(ctx, aqhi, observedAt, raw)
}

func (a *AQHICollector) fetch(ctx context.Context) (aqhi float64, observedAt time.Time, raw []byte, err error) {
	body, err := a.client.Get(ctx, a.apiURL)
	if err != nil {
		return 0, time.Time{}, nil, fmt.Errorf("aqhi fetch: %w", err)
	}

	var resp aqhiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, time.Time{}, nil, fmt.Errorf("aqhi parse: %w", err)
	}
	if len(resp.Features) == 0 {
		return 0, time.Time{}, nil, fmt.Errorf("aqhi: no stations returned for Ottawa bounding box")
	}

	props := resp.Features[0].Properties
	observedAt, err = time.Parse(time.RFC3339, props.ObservationDatetime)
	if err != nil {
		return 0, time.Time{}, nil, fmt.Errorf("aqhi: parse observation_datetime %q: %w", props.ObservationDatetime, err)
	}

	raw, _ = json.Marshal(props)
	return props.AQHI, observedAt, raw, nil
}

func (a *AQHICollector) save(ctx context.Context, aqhi float64, observedAt time.Time, raw []byte) error {
	if err := a.repo.Save(ctx, domain.ExternalSignal{
		SignalName: domain.SignalAQHI,
		Value:      aqhi,
		RawJSON:    raw,
		ObservedAt: observedAt,
		ScrapedAt:  a.clock.Now(),
	}); err != nil {
		a.log.Error("aqhi save failed", "err", err)
	}
	return nil
}
