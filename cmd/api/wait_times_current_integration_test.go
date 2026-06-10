//go:build integration

// Endpoint family: GET /api/v1/hospitals/{id}/wait-times/current.
// Full-stack tests. Shared scaffolding (startStack, testcontainers Postgres,
// seeded hospitals from migrations) lives in main_integration_test.go.
// Test-only helpers (fakeClock, fakeUpstream, body builders, etc.) live in
// integration_helpers_test.go.
//
// Every snapshot or signal in the DB is produced by the real scraper or
// collector code path. Upstream HTTP is stubbed with httptest.NewServer and the
// polite client is built with intervalSec=0 so tests don't sleep. Clock
// injection (internal/clock) pins "now" so trend windows and anomaly baselines
// line up deterministically.

package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adrien2121/GoProject/internal/clock"
	"github.com/adrien2121/GoProject/internal/collector"
	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/httpclient"
	"github.com/adrien2121/GoProject/internal/scraper/sites"
)

func TestEndpoint_HospitalCurrent_404WhenHospitalUnknown(t *testing.T) {
	t.Run(`
		given the API stack is running and no row for "no-such-id" exists,
		when a client GETs /api/v1/hospitals/no-such-id/wait-times/current,
		then the response is 404 because the repo returns ErrNotFound and the
		handler maps it`,
		func(t *testing.T) {
			// Given
			url, _, cleanup := startStack(t)
			defer cleanup()

			// When
			resp, err := http.Get(url + "/api/v1/hospitals/no-such-id/wait-times/current")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()

			// Then
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			if !contains(string(body), "hospital not found") {
				t.Errorf("body = %q, want 'hospital not found' marker", string(body))
			}
		},
	)
}

func TestEndpoint_HospitalCurrent_200NoDataYet(t *testing.T) {
	t.Run(`
		given a seeded hospital "cheo" exists but no scrape has run,
		when a client GETs /api/v1/hospitals/cheo/wait-times/current,
		then the response is 200 with latest absent and trend stable, proving
		the endpoint degrades gracefully when scrapers are cold`,
		func(t *testing.T) {
			// Given
			url, _, cleanup := startStack(t)
			defer cleanup()

			// When
			resp, err := http.Get(url + "/api/v1/hospitals/cheo/wait-times/current")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()

			// Then
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			got := decodeStatus(t, resp)
			// 1. latest absent. json:"latest,omitempty" on a nil pointer must drop the key.
			if _, present := got["latest"]; present {
				t.Errorf("latest key present = %v, want absent when no snapshot exists", got["latest"])
			}
			// 2. trend defaults to stable so the analysis layer never panics on nil latest.
			if got["trend"] != "stable" {
				t.Errorf("trend = %v, want stable", got["trend"])
			}
			// 3. is_unusual false. No baseline means no anomaly.
			if got["is_unusual"] != false {
				t.Errorf("is_unusual = %v, want false", got["is_unusual"])
			}
			// 4. hospital row still serialized so the UI can render the card shell.
			h, ok := got["hospital"].(map[string]any)
			if !ok {
				t.Fatalf("hospital = %v, want object", got["hospital"])
			}
			if h["id"] != "cheo" {
				t.Errorf("hospital.id = %v, want cheo", h["id"])
			}
		},
	)
}

func TestEndpoint_HospitalCurrent_200WithSingleSnapshot(t *testing.T) {
	t.Run(`
		given a fake CHEO upstream returns longestWaitMin=45 and one Scrape() run,
		when a client GETs /api/v1/hospitals/cheo/wait-times/current,
		then the response surfaces wait_minutes=45 with trend stable and
		is_unusual false because n=1 is below the analysis layer's minimum
		history`,
		func(t *testing.T) {
			// Given
			url, store, cleanup := startStack(t)
			defer cleanup()

			now := time.Now().UTC()
			const currentWait = 45
			server := httptest.NewServer(fakeUpstream(t, cheoBody(currentWait)))
			defer server.Close()

			s := sites.NewCHEOScraper(
				httpclient.New(0),
				server.URL,
				newFakeClock(t, now),
			)
			scrapeAndPersist(t, s, store)

			// When
			resp, err := http.Get(url + "/api/v1/hospitals/cheo/wait-times/current")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()
			got := decodeStatus(t, resp)

			// Then
			// 1. latest populated once a snapshot exists, flipping the omitempty branch.
			latest, ok := got["latest"].(map[string]any)
			if !ok {
				t.Fatalf("latest = %v, want object once a snapshot exists", got["latest"])
			}
			// 2. wait_minutes is what the upstream JSON carried, proving the parser pulled longestWaitMin.
			if latest["wait_minutes"].(float64) != float64(currentWait) {
				t.Errorf("latest.wait_minutes = %v, want %d", latest["wait_minutes"], currentWait)
			}
			// 3. category mapped from the scraper's domain constant. Catches a JSON tag typo.
			if latest["category"] != string(domain.WaitCategoryTriageToDoctor) {
				t.Errorf("latest.category = %v, want triage_to_doctor", latest["category"])
			}
			// 4. trend defaults to stable because one row is below computeTrend's minimum history.
			if got["trend"] != "stable" {
				t.Errorf("trend = %v, want stable for single snapshot", got["trend"])
			}
			// 5. is_unusual false because the anomaly baseline is degenerate at n=1.
			if got["is_unusual"] != false {
				t.Errorf("is_unusual = %v, want false for single snapshot", got["is_unusual"])
			}
		},
	)
}

func TestEndpoint_HospitalCurrent_200TrendUp(t *testing.T) {
	t.Run(`
		given the fake CHEO upstream returns three baseline waits of 20 min at
		scraped_at values inside the service's [now-3h, now-1h] window plus a
		current scrape at "now" returning 60 min,
		when a client GETs /api/v1/hospitals/cheo/wait-times/current,
		then trend is "up" because computeTrend's ratio (current over baseline
		avg) clears the 10% threshold`,
		func(t *testing.T) {
			// Given
			url, store, cleanup := startStack(t)
			defer cleanup()

			now := time.Now().UTC()
			// Baseline timestamps sit inside the [now-3h, now-1h] history window.
			// 15-minute gaps mirror the production scraper cadence.
			t1, t2, t3 := now.Add(-2*time.Hour-30*time.Minute), now.Add(-2*time.Hour-15*time.Minute), now.Add(-2 * time.Hour)
			clk := newFakeClock(t, t1, t2, t3, now)

			const (
				baselineWait = 20
				currentWait  = 60
			)
			server := httptest.NewServer(fakeUpstream(t,
				cheoBody(baselineWait), cheoBody(baselineWait), cheoBody(baselineWait),
				cheoBody(currentWait),
			))
			defer server.Close()

			s := sites.NewCHEOScraper(httpclient.New(0), server.URL, clk)
			for range 4 {
				scrapeAndPersist(t, s, store)
			}

			// When
			resp, err := http.Get(url + "/api/v1/hospitals/cheo/wait-times/current")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()

			// Then
			got := decodeStatus(t, resp)
			if got["trend"] != "up" {
				t.Errorf("trend = %v, want up", got["trend"])
			}
		},
	)
}

func TestEndpoint_HospitalCurrent_200TrendDown(t *testing.T) {
	t.Run(`
		given the same setup as the trend-up case but with baseline 60 and
		current 20,
		when a client GETs /api/v1/hospitals/cheo/wait-times/current,
		then trend is "down", symmetric guard on the lower side of the 10% band`,
		func(t *testing.T) {
			// Given
			url, store, cleanup := startStack(t)
			defer cleanup()

			now := time.Now().UTC()
			t1, t2, t3 := now.Add(-2*time.Hour-30*time.Minute), now.Add(-2*time.Hour-15*time.Minute), now.Add(-2 * time.Hour)
			clk := newFakeClock(t, t1, t2, t3, now)

			const (
				baselineWait = 60
				currentWait  = 20
			)
			server := httptest.NewServer(fakeUpstream(t,
				cheoBody(baselineWait), cheoBody(baselineWait), cheoBody(baselineWait),
				cheoBody(currentWait),
			))
			defer server.Close()

			s := sites.NewCHEOScraper(httpclient.New(0), server.URL, clk)
			for range 4 {
				scrapeAndPersist(t, s, store)
			}

			// When
			resp, err := http.Get(url + "/api/v1/hospitals/cheo/wait-times/current")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()

			// Then
			got := decodeStatus(t, resp)
			if got["trend"] != "down" {
				t.Errorf("trend = %v, want down", got["trend"])
			}
		},
	)
}

func TestEndpoint_HospitalCurrent_200IsUnusualWhenAboveBaseline(t *testing.T) {
	t.Run(`
		given seven baseline scrapes at 20 min recorded at the same hour-of-day
		within the last 7 days and a current scrape at 90 min,
		when a client GETs /api/v1/hospitals/cheo/wait-times/current,
		then is_unusual is true because computeAnomaly's 2x baseline-avg
		threshold fires across the full repo and service chain`,
		func(t *testing.T) {
			// Given
			url, store, cleanup := startStack(t)
			defer cleanup()

			now := time.Now().UTC()
			// One-second spacing keeps all eight inside the same wall-clock hour
			// unless "now" is in the last 7 seconds of an hour, which is rare.
			const baselineCount = 7
			times := make([]time.Time, 0, baselineCount+1)
			for i := baselineCount; i >= 1; i-- {
				times = append(times, now.Add(-time.Duration(i)*time.Second))
			}
			times = append(times, now)

			const (
				baselineWait     = 20
				currentSpikeWait = 90
			)
			bodies := make([]string, 0, baselineCount+1)
			for range baselineCount {
				bodies = append(bodies, cheoBody(baselineWait))
			}
			bodies = append(bodies, cheoBody(currentSpikeWait))

			clk := newFakeClock(t, times...)
			server := httptest.NewServer(fakeUpstream(t, bodies...))
			defer server.Close()

			s := sites.NewCHEOScraper(httpclient.New(0), server.URL, clk)
			for range baselineCount + 1 {
				scrapeAndPersist(t, s, store)
			}

			// When
			resp, err := http.Get(url + "/api/v1/hospitals/cheo/wait-times/current")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()

			// Then
			got := decodeStatus(t, resp)
			if got["is_unusual"] != true {
				t.Errorf("is_unusual = %v, want true (current %d vs ~%d baseline ratio > 2.0)",
					got["is_unusual"], currentSpikeWait, baselineWait)
			}
		},
	)
}

func TestEndpoint_HospitalCurrent_200IncludesSignalFromAQHICollector(t *testing.T) {
	t.Run(`
		given one CHEO scrape (so the endpoint has a snapshot to attach signals
		to) and one AQHI collector run against a fake upstream returning
		aqhi=4.2,
		when a client GETs /api/v1/hospitals/cheo/wait-times/current,
		then signals[] contains the regional AQHI signal, proving the collector
		Save path lands rows the service can join through the
		(hospital_id=$1 OR IS NULL) clause and that toStatusResponse attaches
		them`,
		func(t *testing.T) {
			// Given
			url, store, cleanup := startStack(t)
			defer cleanup()

			now := time.Now().UTC()

			// Snapshot first so the endpoint has a "latest" to decorate.
			const currentWait = 45
			snapServer := httptest.NewServer(fakeUpstream(t, cheoBody(currentWait)))
			defer snapServer.Close()
			cheo := sites.NewCHEOScraper(httpclient.New(0), snapServer.URL, newFakeClock(t, now))
			scrapeAndPersist(t, cheo, store)

			// One real AQHI collector pass.
			const aqhiObservedReading = 4.2
			aqhiServer := httptest.NewServer(fakeUpstream(t, aqhiBody(aqhiObservedReading, now)))
			defer aqhiServer.Close()
			aqhi := collector.NewAQHICollector(
				httpclient.New(0),
				aqhiServer.URL,
				clock.RealClock{}, // ScrapedAt = real now is fine. The response asserts on ObservedAt.
				store.externalSignalRepo,
				silentLogger(),
			)
			if err := aqhi.Collect(context.Background()); err != nil {
				t.Fatalf("aqhi collect: %v", err)
			}

			// When
			resp, err := http.Get(url + "/api/v1/hospitals/cheo/wait-times/current")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()
			got := decodeStatus(t, resp)

			// Then
			// 1. signals non-empty so the regional join (hospital_id IS NULL) actually surfaced the row.
			signals, ok := got["signals"].([]any)
			if !ok || len(signals) == 0 {
				t.Fatalf("signals = %v, want non-empty array", got["signals"])
			}
			// 2. AQHI value and signal name match what the collector parsed from the fake upstream.
			found := false
			for _, s := range signals {
				m := s.(map[string]any)
				if m["signal_name"] == string(domain.SignalAQHI) && m["value"].(float64) == aqhiObservedReading {
					found = true
				}
			}
			if !found {
				t.Errorf("AQHI signal not in response signals=%v", signals)
			}
		},
	)
}

func TestEndpoint_HospitalCurrent_200ForInactiveHospital(t *testing.T) {
	t.Run(`
		given the seed migration loaded "toh-civic" as an inactive hospital,
		when a client GETs /api/v1/hospitals/toh-civic/wait-times/current,
		then the response is 200 because the detail endpoint does NOT filter on
		active (only the list endpoint does). Locks in this contract so a
		future "filter everywhere" change is deliberate, not a silent break of
		deep links to closed facilities`,
		func(t *testing.T) {
			// Given
			url, _, cleanup := startStack(t)
			defer cleanup()

			// When
			resp, err := http.Get(url + "/api/v1/hospitals/toh-civic/wait-times/current")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()

			// Then
			// 1. 200, not 404. The detail endpoint doesn't filter on active.
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200 (inactive hospitals still resolve on the detail endpoint)", resp.StatusCode)
			}
			got := decodeStatus(t, resp)
			h, ok := got["hospital"].(map[string]any)
			if !ok {
				t.Fatalf("hospital = %v, want object", got["hospital"])
			}
			// 2. hospital.id matches the requested ID, proving we returned the right row.
			if h["id"] != "toh-civic" {
				t.Errorf("hospital.id = %v, want toh-civic", h["id"])
			}
			// 3. active=false in the payload, confirming it's the inactive row, not a silent substitution.
			if h["active"] != false {
				t.Errorf("hospital.active = %v, want false", h["active"])
			}
		},
	)
}

func TestEndpoint_HospitalCurrent_500WhenPoolClosed(t *testing.T) {
	t.Run(`
		given the API stack is running and the DB pool is closed mid-flight
		(simulating Postgres becoming unreachable),
		when a client GETs /api/v1/hospitals/cheo/wait-times/current,
		then the response is 500 because the repo error is generic (not
		ErrNotFound) and the handler maps it to internal error`,
		func(t *testing.T) {
			// Given
			url, store, cleanup := startStack(t)
			defer cleanup()
			store.db.Close() // real failure injection, same trick used in health_integration_test.go

			// When
			resp, err := http.Get(url + "/api/v1/hospitals/cheo/wait-times/current")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()

			// Then
			if resp.StatusCode != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			if !contains(string(body), "internal error") {
				t.Errorf("body = %q, want 'internal error' marker", string(body))
			}
		},
	)
}
