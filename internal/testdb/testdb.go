//go:build integration

// Package testdb spins a transient Postgres container via testcontainers-go,
// applies the project's real migrations from ./migrations, and exposes helpers
// used by every integration test.
//
// The migrations are the same SQL files the docker-compose `migrate` service runs
// against the production Postgres. Running them here means integration tests see
// the same schema AND the same seeded hospital rows production sees on first boot
// — no test-only Schema constant that can drift away from prod.
//
// Build-tagged so non-integration builds never pull testcontainers' or
// golang-migrate's dependency graph.
package testdb

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // postgres:// URL driver for migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"      // file:// source driver for migrate
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// migrationsPath resolves the project's migrations directory from this file's
// location at runtime. testdb.go lives at internal/testdb/testdb.go so the project
// root sits two levels up.
func migrationsPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")
}

// Start runs a Postgres 16 container, applies every migration under ./migrations
// (schema + seed), and returns the DSN. Callers must invoke the returned cleanup
// func on shutdown so containers never leak between test runs.
func Start(ctx context.Context) (dsn string, cleanup func(), err error) {
	c, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("waittime_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return "", nil, fmt.Errorf("start postgres: %w", err)
	}
	cleanup = func() { _ = c.Terminate(context.Background()) }

	dsn, err = c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("connection string: %w", err)
	}

	if err := applyMigrations(dsn); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("apply migrations: %w", err)
	}
	return dsn, cleanup, nil
}

// applyMigrations runs every up-migration in ./migrations against the given DSN.
// Uses golang-migrate, the same tool the docker-compose `migrate` service runs.
func applyMigrations(dsn string) error {
	m, err := migrate.New("file://"+migrationsPath(), dsn)
	if err != nil {
		return fmt.Errorf("migrate new: %w", err)
	}
	// Close releases the migration driver's source + DB handles. We do not need to
	// keep them open after Up: the caller will open its own pgx pool for queries.
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// TruncateSnapshotsAndSignals wipes the wait_time_snapshots and external_signals
// tables so the next test starts from a clean slate. Hospitals are NOT truncated
// because the seed migration populated them; wiping would force every test to
// re-seed. RESTART IDENTITY resets bigserial counters.
func TruncateSnapshotsAndSignals(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx,
		`TRUNCATE wait_time_snapshots, external_signals RESTART IDENTITY`)
	if err != nil {
		return fmt.Errorf("truncate: %w", err)
	}
	return nil
}
