package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/repository"
)

type hospitalRepo struct {
	db *pgxpool.Pool
}

// NewHospitalRepo returns a PostgreSQL-backed HospitalRepository.
func NewHospitalRepo(db *pgxpool.Pool) repository.HospitalRepository {
	return &hospitalRepo{db: db}
}

func (r *hospitalRepo) GetAll(ctx context.Context) ([]domain.Hospital, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, address, facility_type, source_url, active
		FROM hospitals
		WHERE active = TRUE
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("hospitalRepo.GetAll: %w", err)
	}
	defer rows.Close()

	hospitals, err := pgx.CollectRows(rows, scanHospital)
	if err != nil {
		return nil, fmt.Errorf("hospitalRepo.GetAll collect: %w", err)
	}
	return hospitals, nil
}

func (r *hospitalRepo) GetByID(ctx context.Context, id string) (domain.Hospital, error) {
	var h domain.Hospital
	err := r.db.QueryRow(ctx, `
		SELECT id, name, address, facility_type, source_url, active
		FROM hospitals
		WHERE id = $1
	`, id).Scan(&h.ID, &h.Name, &h.Address, &h.FacilityType, &h.SourceURL, &h.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Hospital{}, fmt.Errorf("hospital %q: %w", id, repository.ErrNotFound)
	}
	if err != nil {
		return domain.Hospital{}, fmt.Errorf("hospitalRepo.GetByID %q: %w", id, err)
	}
	return h, nil
}

func (r *hospitalRepo) Upsert(ctx context.Context, h domain.Hospital) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO hospitals (id, name, address, facility_type, source_url, active, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (id) DO UPDATE SET
			name          = EXCLUDED.name,
			address       = EXCLUDED.address,
			facility_type = EXCLUDED.facility_type,
			source_url    = EXCLUDED.source_url,
			active        = EXCLUDED.active,
			updated_at    = NOW()
	`, h.ID, h.Name, h.Address, h.FacilityType, h.SourceURL, h.Active)
	if err != nil {
		return fmt.Errorf("hospitalRepo.Upsert %q: %w", h.ID, err)
	}
	return nil
}

func scanHospital(row pgx.CollectableRow) (domain.Hospital, error) {
	var h domain.Hospital
	if err := row.Scan(&h.ID, &h.Name, &h.Address, &h.FacilityType, &h.SourceURL, &h.Active); err != nil {
		return h, fmt.Errorf("scan Hospital: %w", err)
	}
	return h, nil
}
