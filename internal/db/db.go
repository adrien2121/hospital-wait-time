// Package db wraps the pgx connection pool with a method set our handlers can
// depend on. Exists so handler.Pinger has a named, grep-able implementer in our
// own codebase instead of relying on Go's structural match against *pgxpool.Pool.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/adrien2121/GoProject/internal/repository/postgres"
)

// DB owns the PostgreSQL pool for the running binary. Both cmd/api and
// cmd/scraper call Open on startup and Close on shutdown.
type DB struct {
	pool *pgxpool.Pool
}

// Open opens a PostgreSQL connection pool. Caller must defer (*DB).Close on shutdown.
func Open(ctx context.Context, dsn string) (*DB, error) {
	pool, err := postgres.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("db.Open: %w", err)
	}
	return &DB{pool: pool}, nil
}

// Ping returns nil if the database is reachable. Implements handler.Pinger.
// Grep for "func (d *DB) Ping" to confirm where the readiness probe ultimately lands.
func (d *DB) Ping(ctx context.Context) error {
	if err := d.pool.Ping(ctx); err != nil {
		return fmt.Errorf("db.Ping: %w", err)
	}
	return nil
}

// Pool exposes the underlying pgx pool to repository constructors that need it.
// Kept narrow on purpose — callers should not start poking at pool internals.
func (d *DB) Pool() *pgxpool.Pool {
	return d.pool
}

// Close releases pool resources. Safe to call on a nil receiver.
func (d *DB) Close() {
	if d != nil && d.pool != nil {
		d.pool.Close()
	}
}
