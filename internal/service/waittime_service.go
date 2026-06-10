package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/repository"
)

type WaitTimeService struct {
	waitRepo     repository.WaitTimeRepository
	hospitalRepo repository.HospitalRepository
	signalRepo   repository.ExternalSignalRepository
}

func NewWaitTimeService(waitRepo repository.WaitTimeRepository, hospitalRepo repository.HospitalRepository, signalRepo repository.ExternalSignalRepository) *WaitTimeService {
	return &WaitTimeService{waitRepo: waitRepo, hospitalRepo: hospitalRepo, signalRepo: signalRepo}
}

// GetAllCurrentStatus returns status for all active hospitals.
func (s *WaitTimeService) GetAllCurrentStatus(ctx context.Context) ([]domain.HospitalStatus, error) {
	hospitals, err := s.hospitalRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get hospitals: %w", err)
	}

	snapshots, err := s.waitRepo.GetAllLatest(ctx)
	if err != nil {
		return nil, fmt.Errorf("get latest snapshots: %w", err)
	}
	// Index by hospitalID for O(1) lookup in the loop below.
	latestByID := make(map[string]domain.WaitTimeSnapshot, len(snapshots))
	for _, snap := range snapshots {
		latestByID[snap.HospitalID] = snap
	}

	statuses := make([]domain.HospitalStatus, 0, len(hospitals))
	for _, h := range hospitals {
		if !h.Active { // skip deactivated hospitals (closed/restructuring)
			continue
		}
		var latest *domain.WaitTimeSnapshot
		if snap, ok := latestByID[h.ID]; ok {
			latest = &snap
		}
		status, err := s.buildStatus(ctx, h, latest)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// GetHospitalStatus returns current status for a single hospital.
func (s *WaitTimeService) GetHospitalStatus(ctx context.Context, id string) (domain.HospitalStatus, error) {
	h, err := s.hospitalRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.HospitalStatus{}, fmt.Errorf("hospital %q: %w", id, repository.ErrNotFound)
		}
		return domain.HospitalStatus{}, fmt.Errorf("get hospital %q: %w", id, err)
	}

	var latest *domain.WaitTimeSnapshot
	snap, err := s.waitRepo.GetLatestByHospital(ctx, id)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return domain.HospitalStatus{}, fmt.Errorf("get latest snapshot %q: %w", id, err)
	}
	if err == nil {
		latest = &snap
	}

	return s.buildStatus(ctx, h, latest)
}

// GetHistory returns snapshots for a hospital in the given time range.
func (s *WaitTimeService) GetHistory(ctx context.Context, hospitalID string, from, to time.Time) ([]domain.WaitTimeSnapshot, error) {
	if !from.Before(to) {
		return nil, fmt.Errorf("from must be before to")
	}
	snaps, err := s.waitRepo.GetHistory(ctx, hospitalID, from, to)
	if err != nil {
		return nil, fmt.Errorf("get history %q: %w", hospitalID, err)
	}
	return snaps, nil
}

// GetBestTimeToVisit returns the historically lowest-wait hour/day for a hospital over 30 days.
func (s *WaitTimeService) GetBestTimeToVisit(ctx context.Context, hospitalID string) (domain.BestTimeSlot, error) {
	since := time.Now().Add(-bestTimeLookback)
	averages, err := s.waitRepo.GetAverageByHourAndDay(ctx, hospitalID, since)
	if err != nil {
		return domain.BestTimeSlot{}, fmt.Errorf("get hourly averages %q: %w", hospitalID, err)
	}
	if len(averages) == 0 {
		return domain.BestTimeSlot{}, fmt.Errorf("hospital %q: %w", hospitalID, repository.ErrNotFound)
	}

	best := averages[0]
	for _, a := range averages[1:] {
		if a.AvgWaitMinutes < best.AvgWaitMinutes {
			best = a
		}
	}

	return domain.BestTimeSlot{
		HospitalID:     hospitalID,
		DayOfWeek:      dayOfWeekName(best.DayOfWeek),
		Hour:           best.Hour,
		AvgWaitMinutes: best.AvgWaitMinutes,
	}, nil
}
