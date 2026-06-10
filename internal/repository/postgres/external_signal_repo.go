package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/repository"
)

type externalSignalRepo struct {
	db *pgxpool.Pool
}

func NewExternalSignalRepo(db *pgxpool.Pool) repository.ExternalSignalRepository {
	return &externalSignalRepo{db: db}
}

func (r *externalSignalRepo) Save(ctx context.Context, s domain.ExternalSignal) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO external_signals (signal_name, hospital_id, value, raw_json, observed_at, scraped_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, s.SignalName, s.HospitalID, s.Value, s.RawJSON, s.ObservedAt, s.ScrapedAt)
	if err != nil {
		return fmt.Errorf("externalSignalRepo.Save %q: %w", s.SignalName, err)
	}
	return nil
}

func (r *externalSignalRepo) GetLatest(ctx context.Context, signalName domain.SignalName) (domain.ExternalSignal, error) {
	var s domain.ExternalSignal
	err := r.db.QueryRow(ctx, `
		SELECT id, signal_name, hospital_id, value, raw_json, observed_at, scraped_at
		FROM external_signals
		WHERE signal_name = $1
		ORDER BY observed_at DESC
		LIMIT 1
	`, string(signalName)).Scan(&s.ID, &s.SignalName, &s.HospitalID, &s.Value, &s.RawJSON, &s.ObservedAt, &s.ScrapedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ExternalSignal{}, fmt.Errorf("signal %q: %w", signalName, repository.ErrNotFound)
	}
	if err != nil {
		return domain.ExternalSignal{}, fmt.Errorf("externalSignalRepo.GetLatest %q: %w", signalName, err)
	}
	return s, nil
}

func (r *externalSignalRepo) GetCurrentSignals(ctx context.Context, hospitalID string) ([]domain.ExternalSignal, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (signal_name)
			id, signal_name, hospital_id, value, raw_json, observed_at, scraped_at
		FROM external_signals
		WHERE hospital_id = $1 OR hospital_id IS NULL
		ORDER BY signal_name, observed_at DESC
	`, hospitalID)
	if err != nil {
		return nil, fmt.Errorf("externalSignalRepo.GetCurrentSignals %q: %w", hospitalID, err)
	}
	defer rows.Close()

	signals, err := pgx.CollectRows(rows, scanExternalSignal)
	if err != nil {
		return nil, fmt.Errorf("externalSignalRepo.GetCurrentSignals %q collect: %w", hospitalID, err)
	}
	return signals, nil
}

func (r *externalSignalRepo) GetForHospital(ctx context.Context, hospitalID string, since time.Time) ([]domain.ExternalSignal, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, signal_name, hospital_id, value, raw_json, observed_at, scraped_at
		FROM external_signals
		WHERE (hospital_id = $1 OR hospital_id IS NULL)
		  AND observed_at >= $2
		ORDER BY signal_name, observed_at DESC
	`, hospitalID, since)
	if err != nil {
		return nil, fmt.Errorf("externalSignalRepo.GetForHospital %q: %w", hospitalID, err)
	}
	defer rows.Close()

	signals, err := pgx.CollectRows(rows, scanExternalSignal)
	if err != nil {
		return nil, fmt.Errorf("externalSignalRepo.GetForHospital %q collect: %w", hospitalID, err)
	}
	return signals, nil
}

func scanExternalSignal(row pgx.CollectableRow) (domain.ExternalSignal, error) {
	var s domain.ExternalSignal
	if err := row.Scan(&s.ID, &s.SignalName, &s.HospitalID, &s.Value, &s.RawJSON, &s.ObservedAt, &s.ScrapedAt); err != nil {
		return s, fmt.Errorf("scan ExternalSignal: %w", err)
	}
	return s, nil
}
