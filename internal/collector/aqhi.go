package collector

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
)

// AQHIURL is the production upstream. Exported so production wiring and
// integration tests name the same URL (tests substitute an httptest.Server URL
// via the constructor).
// Ottawa bounding box; latest=true returns the single most recent AQHI reading.
const AQHIURL = "https://api.weather.gc.ca/collections/aqhi-observations-realtime/items?f=json&limit=1&bbox=-75.9,45.2,-75.4,45.5&latest=true"

const aqhiInterval = 1 * time.Hour

// AQHICollector polls Environment Canada's real-time AQHI endpoint.
// AQHI (Air Quality Health Index) updates hourly on a 1–10+ scale.
//
// Lifecycle (jitter, timeout, backoff, logging) lives in runner.Run.
// This struct only owns the fetch/parse/save shape specific to AQHI.
type AQHICollector struct {
	runner.Base
	client httpclient.Getter
	apiURL string
	clock  clock.Clock
	repo   repository.ExternalSignalRepository
	log    *slog.Logger
}

// Compile-time check: AQHICollector must satisfy runner.Runnable.
var _ runner.Runnable = (*AQHICollector)(nil)

// NewAQHICollector wires the AQHI collector.
func NewAQHICollector(client httpclient.Getter, apiURL string, c clock.Clock, repo repository.ExternalSignalRepository, log *slog.Logger) *AQHICollector {
	return &AQHICollector{
		Base:   runner.NewBase("aqhi", aqhiInterval),
		client: client,
		apiURL: apiURL,
		clock:  c,
		repo:   repo,
		log:    log,
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

// Run performs one fetch+save pass.
func (a *AQHICollector) Run(ctx context.Context) error {
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
	} else {
		a.log.Info("aqhi signal ok", "value", aqhi, "observed_at", observedAt.Format(time.RFC3339))
	}
	return nil
}
