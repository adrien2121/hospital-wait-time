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

type waitTimeRepo struct {
	db *pgxpool.Pool
}

// NewWaitTimeRepo returns a PostgreSQL-backed WaitTimeRepository.
func NewWaitTimeRepo(db *pgxpool.Pool) repository.WaitTimeRepository {
	return &waitTimeRepo{db: db}
}

func (r *waitTimeRepo) Save(ctx context.Context, s domain.WaitTimeSnapshot) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO wait_time_snapshots (hospital_id, wait_minutes, category, recorded_at, scraped_at)
		VALUES ($1, $2, $3, $4, $5)
	`, s.HospitalID, s.WaitMinutes, s.Category, s.RecordedAt, s.ScrapedAt)
	if err != nil {
		return fmt.Errorf("waitTimeRepo.Save hospital %q: %w", s.HospitalID, err)
	}
	return nil
}

func (r *waitTimeRepo) GetLatestByHospital(ctx context.Context, hospitalID string) (domain.WaitTimeSnapshot, error) {
	var s domain.WaitTimeSnapshot
	err := r.db.QueryRow(ctx, `
		SELECT id, hospital_id, wait_minutes, category, recorded_at, scraped_at
		FROM wait_time_snapshots
		WHERE hospital_id = $1
		ORDER BY scraped_at DESC
		LIMIT 1
	`, hospitalID).Scan(&s.ID, &s.HospitalID, &s.WaitMinutes, &s.Category, &s.RecordedAt, &s.ScrapedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WaitTimeSnapshot{}, fmt.Errorf("wait time for hospital %q: %w", hospitalID, repository.ErrNotFound)
	}
	if err != nil {
		return domain.WaitTimeSnapshot{}, fmt.Errorf("waitTimeRepo.GetLatestByHospital %q: %w", hospitalID, err)
	}
	return s, nil
}

func (r *waitTimeRepo) GetAllLatest(ctx context.Context) ([]domain.WaitTimeSnapshot, error) {
	// DISTINCT ON returns the most recent snapshot per hospital, ordered by scraped_at DESC.
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (hospital_id) id, hospital_id, wait_minutes, category, recorded_at, scraped_at
		FROM wait_time_snapshots
		ORDER BY hospital_id, scraped_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("waitTimeRepo.GetAllLatest: %w", err)
	}
	defer rows.Close()

	snapshots, err := pgx.CollectRows(rows, scanWaitTimeSnapshot)
	if err != nil {
		return nil, fmt.Errorf("waitTimeRepo.GetAllLatest collect: %w", err)
	}
	return snapshots, nil
}

func (r *waitTimeRepo) GetHistory(ctx context.Context, hospitalID string, from, to time.Time) ([]domain.WaitTimeSnapshot, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, hospital_id, wait_minutes, category, recorded_at, scraped_at
		FROM wait_time_snapshots
		WHERE hospital_id = $1
		  AND scraped_at BETWEEN $2 AND $3
		ORDER BY scraped_at ASC
	`, hospitalID, from, to)
	if err != nil {
		return nil, fmt.Errorf("waitTimeRepo.GetHistory hospital %q: %w", hospitalID, err)
	}
	defer rows.Close()

	snapshots, err := pgx.CollectRows(rows, scanWaitTimeSnapshot)
	if err != nil {
		return nil, fmt.Errorf("waitTimeRepo.GetHistory hospital %q collect: %w", hospitalID, err)
	}
	return snapshots, nil
}

// GetAverageByHourAndDay takes an absolute `since` time (set by the caller from
// its own time.Now()) so the repo stays a thin SQL adapter, no wall clock reads.
func (r *waitTimeRepo) GetAverageByHourAndDay(ctx context.Context, hospitalID string, since time.Time) ([]repository.HourlyAverage, error) {
	// AT TIME ZONE 'UTC' for the same reason documented on GetAverageForWindow:
	// EXTRACT on a timestamptz honors the session TimeZone and would otherwise
	// bucket by local hour, breaking comparability across deployments.
	rows, err := r.db.Query(ctx, `
		SELECT
			hospital_id,
			EXTRACT(dow  FROM recorded_at AT TIME ZONE 'UTC')::int AS day_of_week,
			EXTRACT(hour FROM recorded_at AT TIME ZONE 'UTC')::int AS hour,
			AVG(wait_minutes)::float8                              AS avg_wait_minutes
		FROM wait_time_snapshots
		WHERE hospital_id = $1
		  AND recorded_at >= $2
		GROUP BY hospital_id, day_of_week, hour
		ORDER BY day_of_week, hour
	`, hospitalID, since)
	if err != nil {
		return nil, fmt.Errorf("waitTimeRepo.GetAverageByHourAndDay hospital %q: %w", hospitalID, err)
	}
	defer rows.Close()

	averages, err := pgx.CollectRows(rows, scanHourlyAverage)
	if err != nil {
		return nil, fmt.Errorf("waitTimeRepo.GetAverageByHourAndDay hospital %q collect: %w", hospitalID, err)
	}
	return averages, nil
}

// GetAverageForWindow takes absolute `since` so the repo never reads the clock.
// referenceTime supplies the hour and day-of-week to match against.
//
// dow and hour are computed in UTC on both sides. The Go side calls UTC()
// before Weekday/Hour. The SQL side forces UTC with `AT TIME ZONE 'UTC'`
// because EXTRACT on a timestamptz uses the session TimeZone, which may differ
// across environments (driver default Local, Postgres session UTC) and would
// silently zero the result.
func (r *waitTimeRepo) GetAverageForWindow(ctx context.Context, hospitalID string, since time.Time, referenceTime time.Time) (float64, error) {
	ref := referenceTime.UTC()
	dow := int(ref.Weekday()) // 0=Sunday, matches PostgreSQL EXTRACT(dow)
	hour := ref.Hour()

	var avg float64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(AVG(wait_minutes), 0)
		FROM wait_time_snapshots
		WHERE hospital_id = $1
		  AND recorded_at >= $2
		  AND EXTRACT(dow  FROM recorded_at AT TIME ZONE 'UTC')::int = $3
		  AND EXTRACT(hour FROM recorded_at AT TIME ZONE 'UTC')::int = $4
	`, hospitalID, since, dow, hour).Scan(&avg)
	if err != nil {
		return 0, fmt.Errorf("waitTimeRepo.GetAverageForWindow hospital %q: %w", hospitalID, err)
	}
	return avg, nil
}

// scanWaitTimeSnapshot is the row mapper used by pgx.CollectRows.
// Lives here (not in domain) so domain stays free of pgx imports.
func scanWaitTimeSnapshot(row pgx.CollectableRow) (domain.WaitTimeSnapshot, error) {
	var s domain.WaitTimeSnapshot
	if err := row.Scan(&s.ID, &s.HospitalID, &s.WaitMinutes, &s.Category, &s.RecordedAt, &s.ScrapedAt); err != nil {
		return s, fmt.Errorf("scan WaitTimeSnapshot: %w", err)
	}
	return s, nil
}

func scanHourlyAverage(row pgx.CollectableRow) (repository.HourlyAverage, error) {
	var a repository.HourlyAverage
	if err := row.Scan(&a.HospitalID, &a.DayOfWeek, &a.Hour, &a.AvgWaitMinutes); err != nil {
		return a, fmt.Errorf("scan HourlyAverage: %w", err)
	}
	return a, nil
}
