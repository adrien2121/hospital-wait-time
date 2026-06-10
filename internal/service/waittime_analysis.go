package service

import (
	"context"
	"fmt"
	"time"

	"github.com/adrien2121/GoProject/internal/domain"
)

const (
	trendThreshold    = 0.10 // 10% change triggers up/down
	anomalyMultiplier = 2.0  // wait > 2× normal avg → IsUnusual
	anomalyWindow     = 7 * 24 * time.Hour
	bestTimeLookback  = 30 * 24 * time.Hour
	trendLookback     = 2 * time.Hour // history window for trend comparison
)

// buildStatus computes trend and anomaly for one hospital given an optional latest snapshot.
// latest is nil when the hospital exists but has not been scraped yet.
func (s *WaitTimeService) buildStatus(ctx context.Context, h domain.Hospital, latest *domain.WaitTimeSnapshot) (domain.HospitalStatus, error) {
	if latest == nil {
		return domain.HospitalStatus{Hospital: h, Trend: domain.TrendStable}, nil
	}

	trend, err := s.computeTrend(ctx, h.ID, *latest)
	if err != nil {
		trend = domain.TrendStable // insufficient history: default stable, still return status
	}

	isUnusual, err := s.computeAnomaly(ctx, h.ID, *latest)
	if err != nil {
		isUnusual = false // no baseline data yet: skip anomaly flag, still return status
	}

	// TODO: once enough historical signal+wait data exists, replace this with a
	// regression model that adjusts WaitMinutes based on signals (AQHI, weather).
	// For now, signals are returned as context only — they don't affect the number.
	signals, err := s.signalRepo.GetCurrentSignals(ctx, h.ID)
	if err != nil {
		signals = nil // non-critical: still return status without signals
	}

	return domain.HospitalStatus{
		Hospital:   h,
		Latest:     latest,
		Trend:      trend,
		IsUnusual:  isUnusual,
		ScrapedAgo: time.Since(latest.ScrapedAt),
		Signals:    signals,
	}, nil
}

// computeTrend compares current wait to the average 1–3 hours prior.
func (s *WaitTimeService) computeTrend(ctx context.Context, hospitalID string, current domain.WaitTimeSnapshot) (domain.TrendDirection, error) {
	// Window: 3h ago → 1h ago relative to current snapshot.
	to := current.ScrapedAt.Add(-1 * time.Hour)
	from := to.Add(-trendLookback)
	history, err := s.waitRepo.GetHistory(ctx, hospitalID, from, to)
	if err != nil {
		return domain.TrendStable, fmt.Errorf("get trend history: %w", err)
	}
	if len(history) == 0 {
		return domain.TrendStable, nil
	}

	var sum int
	for _, h := range history {
		sum += h.WaitMinutes
	}
	avg := float64(sum) / float64(len(history))

	ratio := float64(current.WaitMinutes) / avg
	switch {
	case ratio > 1+trendThreshold:
		return domain.TrendUp, nil
	case ratio < 1-trendThreshold:
		return domain.TrendDown, nil
	default:
		return domain.TrendStable, nil
	}
}

// computeAnomaly flags current wait > 2× the 7-day avg for the same hour and day-of-week.
func (s *WaitTimeService) computeAnomaly(ctx context.Context, hospitalID string, current domain.WaitTimeSnapshot) (bool, error) {
	since := time.Now().Add(-anomalyWindow)
	avg, err := s.waitRepo.GetAverageForWindow(ctx, hospitalID, since, current.RecordedAt)
	if err != nil {
		return false, fmt.Errorf("get anomaly baseline: %w", err)
	}
	if avg == 0 {
		return false, nil
	}
	return float64(current.WaitMinutes) > anomalyMultiplier*avg, nil
}

// dayOfWeekName maps a PostgreSQL EXTRACT(dow) integer (0=Sunday) to its English name
// via time.Weekday, the canonical stdlib mapping.
func dayOfWeekName(dow int) string {
	if dow < 0 || dow > 6 {
		return "Unknown"
	}
	return time.Weekday(dow).String()
}
