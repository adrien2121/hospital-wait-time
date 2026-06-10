package main

import (
	"context"
	"fmt"

	"github.com/adrien2121/GoProject/internal/db"
	"github.com/adrien2121/GoProject/internal/handler"
	"github.com/adrien2121/GoProject/internal/repository"
	"github.com/adrien2121/GoProject/internal/repository/postgres"
)

// Compile-time assertion: *db.DB must satisfy handler.Pinger. If pgx changes
// pgxpool.Pool.Ping's signature (and DB.Ping along with it), this line breaks
// the build instead of letting the readiness handler fail at runtime.
var _ handler.Pinger = (*db.DB)(nil)

// apiStorage groups the DB handle and the repositories the API binary needs.
// Lives in cmd/api because only this binary uses this exact bundle.
type apiStorage struct {
	db                 *db.DB
	hospitalRepo       repository.HospitalRepository
	waitTimeRepo       repository.WaitTimeRepository
	externalSignalRepo repository.ExternalSignalRepository
}

// openAPIStorage opens a PostgreSQL pool and builds the repositories the API needs.
// Caller must defer (*apiStorage).Close on shutdown.
func openAPIStorage(ctx context.Context, dsn string) (*apiStorage, error) {
	d, err := db.Open(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("openAPIStorage: %w", err)
	}
	pool := d.Pool()
	return &apiStorage{
		db:                 d,
		hospitalRepo:       postgres.NewHospitalRepo(pool),
		waitTimeRepo:       postgres.NewWaitTimeRepo(pool),
		externalSignalRepo: postgres.NewExternalSignalRepo(pool),
	}, nil
}

// Close releases all storage resources. Safe to call on a nil receiver.
func (s *apiStorage) Close() {
	if s != nil {
		s.db.Close()
	}
}
