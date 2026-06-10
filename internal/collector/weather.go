// Package collector contains background collectors for external signals (weather, events, etc.)
// used to enrich wait time estimates when hospitals don't publish live data.
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
	// SWOBURL is the production upstream. Exported so production wiring and
	// integration tests name the same URL.
	// Bounding box covers Ottawa proper. Returns the nearest SWOB stations.
	SWOBURL         = "https://api.weather.gc.ca/collections/swob-realtime/items?f=json&limit=5&bbox=-75.9,45.2,-75.4,45.5&sortby=-date_tm-value"
	weatherInterval = 30 * time.Minute
	weatherTimeout  = 30 * time.Second
)

// WeatherCollector polls Environment Canada's SWOB real-time feed.
// Stores temperature and precipitation as ExternalSignals with no hospital_id
// since weather is regional and applies to all hospitals equally.
type WeatherCollector struct {
	client HTTPGetter
	apiURL string
	clock  clock.Clock
	repo   repository.ExternalSignalRepository
	log    *slog.Logger
}

// NewWeatherCollector wires the SWOB collector.
func NewWeatherCollector(client HTTPGetter, apiURL string, c clock.Clock, repo repository.ExternalSignalRepository, log *slog.Logger) *WeatherCollector {
	return &WeatherCollector{client: client, apiURL: apiURL, clock: c, repo: repo, log: log}
}

// Run loops forever collecting weather every weatherInterval.
// The only exit is ctx cancellation (SIGINT/SIGTERM), picked up by the select
// inside the loop. Same lifecycle pattern as the scraper orchestrator.
func (w *WeatherCollector) Run(ctx context.Context) {
	for {
		collectCtx, cancel := context.WithTimeout(ctx, weatherTimeout)
		if err := w.Collect(collectCtx); err != nil {
			w.log.Error("weather collect failed", "err", err)
		}
		cancel() // release the WithTimeout timer now that collect returned; not calling it leaks a goroutine per loop
		select {
		case <-ctx.Done():
			return
		case <-time.After(weatherInterval):
		}
	}
}

// swobResponse mirrors the GeoJSON envelope returned by the EC SWOB endpoint.
type swobResponse struct {
	Features []struct {
		Properties map[string]any `json:"properties"`
	} `json:"features"`
}

// Collect performs one fetch+save pass.
func (w *WeatherCollector) Collect(ctx context.Context) error {
	props, raw, err := w.fetch(ctx)
	if err != nil {
		return err
	}
	return w.save(ctx, props, raw)
}

// fetch retrieves and parses the SWOB response, returning the freshest station's
// properties and its raw JSON for storage.
func (w *WeatherCollector) fetch(ctx context.Context) (props map[string]any, raw []byte, err error) {
	body, err := w.client.Get(ctx, w.apiURL)
	if err != nil {
		return nil, nil, fmt.Errorf("weather fetch: %w", err)
	}

	var resp swobResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("weather parse: %w", err)
	}
	if len(resp.Features) == 0 {
		return nil, nil, fmt.Errorf("weather: no stations returned for Ottawa bounding box")
	}

	// sortby=-date_tm-value puts the freshest observation first.
	props = resp.Features[0].Properties
	raw, _ = json.Marshal(props)
	return props, raw, nil
}

// save extracts known signal fields from station properties and persists each one.
// Not all stations report all fields; missing or non-numeric keys are skipped.
func (w *WeatherCollector) save(ctx context.Context, props map[string]any, raw []byte) error {
	signals := []struct {
		name domain.SignalName
		key  string
	}{
		{domain.SignalWeatherTempC, "air_temp"},
		{domain.SignalWeatherPrecipMM, "pcpn-amt_pst1hr"},
		{domain.SignalWeatherSnowCM, "sno_cu_dwl"},
	}

	now := w.clock.Now()

	for _, sig := range signals {
		v, ok := props[sig.key]
		if !ok {
			w.log.Debug("weather signal key missing", "signal", sig.name, "key", sig.key)
			continue
		}
		val, ok := toFloat(v)
		if !ok {
			w.log.Debug("weather signal non-numeric", "signal", sig.name, "key", sig.key, "value", v)
			continue
		}
		if err := w.repo.Save(ctx, domain.ExternalSignal{
			SignalName: sig.name,
			Value:      val,
			RawJSON:    raw,
			ObservedAt: now,
			ScrapedAt:  now,
		}); err != nil {
			w.log.Error("weather save failed", "signal", sig.name, "err", err)
		}
	}
	return nil
}

// toFloat handles the three numeric types json.Unmarshal can produce
// when decoding into map[string]any.
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}
