package repository

import (
	"context"
	"time"

	"github.com/adrien2121/GoProject/internal/domain"
)

type ExternalSignalRepository interface {
	Save(ctx context.Context, signal domain.ExternalSignal) error
	// Lets the signal collector check the last observation time to avoid redundant fetches.
	GetLatest(ctx context.Context, signalName domain.SignalName) (domain.ExternalSignal, error)
	// Feeds the estimator: pulls all signals relevant to one hospital. Both
	// hospital-specific and regional (nil hospital_id), for the given time window.
	GetForHospital(ctx context.Context, hospitalID string, since time.Time) ([]domain.ExternalSignal, error)
	// Returns the most recent observation per signal name for a hospital.
	// Includes regional signals (nil hospital_id). Used to enrich the current-status response.
	GetCurrentSignals(ctx context.Context, hospitalID string) ([]domain.ExternalSignal, error)
}
