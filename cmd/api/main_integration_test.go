//go:build integration

// Tier 3: full-stack endpoint tests. Real Postgres (testcontainers), real wiring
// (openAPIStorage + buildMux), real HTTP socket (httptest.NewServer), real http.Client.
// Force errors against real components: close the pool to simulate DB-down, etc.
//
// Run with: go test -tags=integration ./cmd/api/...
//
// This file owns the shared infrastructure (container DSN, TestMain, stack-bootstrap
// helper) used by the endpoint-family-specific files in the same package
// (health_integration_test.go, hospitals_integration_test.go, etc.).

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/adrien2121/GoProject/internal/testdb"
)

// sharedDSN is the DSN of the Postgres container shared across every test in this
// package. Each test still opens its own *apiStorage from that DSN so a test can
// close its own pool without affecting siblings (used by the 503-readiness case).
var sharedDSN string

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn, cleanup, err := testdb.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration tests skipped: %v\n", err)
		os.Exit(0)
	}
	defer cleanup()
	sharedDSN = dsn
	os.Exit(m.Run())
}

// startStack boots the same wiring the production binary uses: openAPIStorage,
// buildMux, and httptest.NewServer wrapping the mux. Returns the live URL and
// a cleanup func the test must defer. Each test gets its own storage and server
// so closing one pool does not affect another test.
func startStack(t *testing.T) (baseURL string, store *apiStorage, cleanup func()) {
	t.Helper()
	if sharedDSN == "" {
		t.Skip("integration: sharedDSN unset (TestMain skipped)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s, err := openAPIStorage(ctx, sharedDSN)
	if err != nil {
		t.Fatalf("openAPIStorage: %v", err)
	}

	// Wipe snapshot + signal rows so each test starts from a clean state.
	// Hospitals are NOT touched: they were inserted by the seed migration and
	// must remain so tests see the same baseline production does.
	if err := testdb.TruncateSnapshotsAndSignals(context.Background(), s.db.Pool()); err != nil {
		s.Close()
		t.Fatalf("truncate: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil)) // silence test output
	hospitalSvc, waitTimeSvc := buildServices(s.hospitalRepo, s.waitTimeRepo, s.externalSignalRepo)
	mux := buildMux(hospitalSvc, waitTimeSvc, s.db, logger)

	srv := httptest.NewServer(mux)
	return srv.URL, s, func() {
		srv.Close()
		s.Close()
	}
}

// contains is a tiny strings.Contains alias kept local so endpoint test files
// can stay free of the strings import.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
