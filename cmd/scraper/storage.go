package main

import (
	"context"
	"fmt"

	"github.com/adrien2121/GoProject/internal/db"
	"github.com/adrien2121/GoProject/internal/repository"
	"github.com/adrien2121/GoProject/internal/repository/postgres"
)

// scraperStorage groups the DB handle and the repositories the scraper binary needs.
// HospitalRepo is intentionally absent — the scraper never reads hospital metadata
// at runtime, and there is no readiness endpoint.
type scraperStorage struct {
	db                 *db.DB
	waitTimeRepo       repository.WaitTimeRepository
	externalSignalRepo repository.ExternalSignalRepository
}

// openScraperStorage opens a PostgreSQL pool and builds the repositories the scraper needs.
// Caller must defer (*scraperStorage).Close on shutdown.
func openScraperStorage(ctx context.Context, dsn string) (*scraperStorage, error) {
	d, err := db.Open(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("openScraperStorage: %w", err)
	}
	pool := d.Pool()
	return &scraperStorage{
		db:                 d,
		waitTimeRepo:       postgres.NewWaitTimeRepo(pool),
		externalSignalRepo: postgres.NewExternalSignalRepo(pool),
	}, nil
}

// Close releases all storage resources. Safe to call on a nil receiver.
func (s *scraperStorage) Close() {
	if s != nil {
		s.db.Close()
	}
}
