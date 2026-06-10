//go:build integration

// Shared helpers for endpoint integration tests in this package.
// Kept in their own file so the *_integration_test.go files stay focused on
// scenarios, not plumbing. Build-tagged so they only compile under the
// integration build like every other Tier-3 file here.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/adrien2121/GoProject/internal/scraper"
)

// fakeClock returns successive pinned times. Each Now() call advances to the
// next entry, mirroring the order in which the scraper / collector calls it.
// Fails the test if the code under test asks for more times than were prepared.
type fakeClock struct {
	t     *testing.T
	times []time.Time
	i     int
}

func newFakeClock(t *testing.T, times ...time.Time) *fakeClock {
	t.Helper()
	return &fakeClock{t: t, times: times}
}

func (c *fakeClock) Now() time.Time {
	c.t.Helper()
	if c.i >= len(c.times) {
		c.t.Fatalf("fakeClock exhausted: code under test called Now() more than %d times", len(c.times))
	}
	t := c.times[c.i]
	c.i++
	return t
}

// fakeUpstream builds an http.HandlerFunc that serves each body in turn (one
// per inbound request) so a scraper invoked multiple times sees a fresh
// response each call. Wrap it with httptest.NewServer at the call site.
func fakeUpstream(t *testing.T, bodies ...string) http.HandlerFunc {
	t.Helper()
	i := 0
	return func(w http.ResponseWriter, _ *http.Request) {
		if i >= len(bodies) {
			t.Errorf("fakeUpstream exhausted after %d responses", len(bodies))
			http.Error(w, "exhausted", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(bodies[i]))
		i++
	}
}

// scrapeAndPersist drives one Scrape() and persists the resulting snapshots.
// The scraper produces the snapshot; the test persists it because
// Orchestrator.Run is an infinite goroutine loop unsuitable for tests.
func scrapeAndPersist(t *testing.T, s scraper.Scraper, store *apiStorage) {
	t.Helper()
	snaps, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	for _, snap := range snaps {
		if err := store.waitTimeRepo.Save(context.Background(), snap); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
}

// decodeStatus pulls a JSON body into a loose map so tests assert on individual
// keys without coupling to the unexported statusResponse type.
func decodeStatus(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (body=%q)", err, string(body))
	}
	return got
}

// silentLogger discards collector log output so tests don't spam stderr.
func silentLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// cheoBody is a JSON envelope matching the live CHEO endpoint's shape so a
// regression in the parser surfaces here, not only in production.
func cheoBody(waitMin int) string {
	return fmt.Sprintf(`{"longestWaitMin": %d, "aveWaitMin": %d, "patientCount": 12}`, waitMin, waitMin)
}

// aqhiBody is a JSON envelope matching Environment Canada's AQHI shape so the
// collector parser is exercised end-to-end.
func aqhiBody(aqhi float64, observedAt time.Time) string {
	return fmt.Sprintf(`{
		"features": [{
			"properties": {
				"aqhi": %g,
				"observation_datetime": %q
			}
		}]
	}`, aqhi, observedAt.Format(time.RFC3339))
}
