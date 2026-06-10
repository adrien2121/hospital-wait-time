package repository

import (
	"context"
	"time"

	"github.com/adrien2121/GoProject/internal/domain"
)

// WaitTimeRepository defines data access for WaitTimeSnapshot entities.
//
// Time-window methods take absolute time.Time bounds instead of durations.
// Reason: if the repo called time.Now() itself, tests would have to freeze the
// system clock to assert exact SQL params, two methods invoked in the same
// request would drift apart by a few ms, and an NTP correction mid-call could
// shift the window. Callers (services) read the clock once at the request
// boundary and pass that value down, so every method sees the same `now`.
type WaitTimeRepository interface {
	Save(ctx context.Context, snapshot domain.WaitTimeSnapshot) error
	GetLatestByHospital(ctx context.Context, hospitalID string) (domain.WaitTimeSnapshot, error)
	GetAllLatest(ctx context.Context) ([]domain.WaitTimeSnapshot, error)
	GetHistory(ctx context.Context, hospitalID string, from, to time.Time) ([]domain.WaitTimeSnapshot, error)
	// GetAverageByHourAndDay returns avg wait minutes grouped by hour-of-day and
	// day-of-week for the given hospital over [since, now]. Used for best-time analysis.
	GetAverageByHourAndDay(ctx context.Context, hospitalID string, since time.Time) ([]HourlyAverage, error)
	// GetAverageForWindow returns avg wait minutes for a hospital since the given time,
	// filtered to the same hour and day-of-week as referenceTime. Used for anomaly detection.
	GetAverageForWindow(ctx context.Context, hospitalID string, since time.Time, referenceTime time.Time) (float64, error)
}
