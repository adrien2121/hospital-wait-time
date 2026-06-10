//go:build integration

// Endpoint family: liveness + readiness probes.
// Full-stack tests; see main_integration_test.go for shared scaffolding.

package main

import (
	"io"
	"net/http"
	"testing"
)

func TestEndpoint_HealthAlwaysReturns200(t *testing.T) {
	t.Run(`
		given the full API stack is running,
		when a client GETs /health,
		then it receives 200 OK with a JSON {"status":"ok"} body
		(no DB involvement; liveness must not depend on the database)`,
		func(t *testing.T) {
			// Given: the real stack is up.
			url, _, cleanup := startStack(t)
			defer cleanup()

			// When: a real HTTP client hits /health.
			resp, err := http.Get(url + "/health")
			if err != nil {
				t.Fatalf("GET /health: %v", err)
			}
			defer resp.Body.Close()

			// Then: 200 with ok body.
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			if !contains(string(body), `"status":"ok"`) {
				t.Fatalf("body = %q, want ok marker", string(body))
			}
		},
	)
}

func TestEndpoint_Ready_200WhenPostgresReachable(t *testing.T) {
	t.Run(`
		given the full API stack is running against a healthy Postgres,
		when a client GETs /ready,
		then it receives 200 OK because db.Ping against the real container succeeds`,
		func(t *testing.T) {
			// Given: full stack with a healthy DB.
			url, _, cleanup := startStack(t)
			defer cleanup()

			// When: a real HTTP client hits /ready.
			resp, err := http.Get(url + "/ready")
			if err != nil {
				t.Fatalf("GET /ready: %v", err)
			}
			defer resp.Body.Close()

			// Then: 200.
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
		},
	)
}

func TestEndpoint_Ready_503WhenPoolClosed(t *testing.T) {
	t.Run(`
		given the full API stack is running, but the DB pool has been closed
		(simulating Postgres becoming unreachable mid-flight),
		when a client GETs /ready,
		then it receives 503 because db.Ping against the real (now-closed) pool fails`,
		func(t *testing.T) {
			// Given: full stack up, then close the pool to simulate DB outage.
			url, store, cleanup := startStack(t)
			defer cleanup()
			store.db.Close() // real failure injection: pgxpool.Pool.Ping returns "closed pool"

			// When: a real HTTP client hits /ready.
			resp, err := http.Get(url + "/ready")
			if err != nil {
				t.Fatalf("GET /ready: %v", err)
			}
			defer resp.Body.Close()

			// Then: 503.
			if resp.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", resp.StatusCode)
			}
		},
	)
}
