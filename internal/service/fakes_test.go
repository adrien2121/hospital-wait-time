package service

import (
	"context"
	"time"

	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/repository"
)

// Fakes for the three repository interfaces. Each field is the canned return value
// for the matching method, so a test just sets the fields it cares about and ignores
// the rest. Keeps tests focused on the service's logic, not on a mocking DSL.

type fakeHospitalRepo struct {
	all  []domain.Hospital
	byID map[string]domain.Hospital
	err  error
}

func (f *fakeHospitalRepo) GetAll(_ context.Context) ([]domain.Hospital, error) {
	return f.all, f.err
}

func (f *fakeHospitalRepo) GetByID(_ context.Context, id string) (domain.Hospital, error) {
	if f.err != nil {
		return domain.Hospital{}, f.err
	}
	h, ok := f.byID[id]
	if !ok {
		return domain.Hospital{}, repository.ErrNotFound
	}
	return h, nil
}

func (f *fakeHospitalRepo) Upsert(_ context.Context, _ domain.Hospital) error { return f.err }

type fakeWaitTimeRepo struct {
	saved      []domain.WaitTimeSnapshot
	latest     domain.WaitTimeSnapshot
	latestErr  error
	allLatest  []domain.WaitTimeSnapshot
	history    []domain.WaitTimeSnapshot
	historyErr error
	hourly     []repository.HourlyAverage
	hourlyErr  error
	windowAvg  float64
	windowErr  error
}

func (f *fakeWaitTimeRepo) Save(_ context.Context, s domain.WaitTimeSnapshot) error {
	f.saved = append(f.saved, s)
	return nil
}

func (f *fakeWaitTimeRepo) GetLatestByHospital(_ context.Context, _ string) (domain.WaitTimeSnapshot, error) {
	return f.latest, f.latestErr
}

func (f *fakeWaitTimeRepo) GetAllLatest(_ context.Context) ([]domain.WaitTimeSnapshot, error) {
	return f.allLatest, nil
}

func (f *fakeWaitTimeRepo) GetHistory(_ context.Context, _ string, _, _ time.Time) ([]domain.WaitTimeSnapshot, error) {
	return f.history, f.historyErr
}

func (f *fakeWaitTimeRepo) GetAverageByHourAndDay(_ context.Context, _ string, _ time.Time) ([]repository.HourlyAverage, error) {
	return f.hourly, f.hourlyErr
}

func (f *fakeWaitTimeRepo) GetAverageForWindow(_ context.Context, _ string, _ time.Time, _ time.Time) (float64, error) {
	return f.windowAvg, f.windowErr
}

type fakeSignalRepo struct {
	current    []domain.ExternalSignal
	currentErr error
}

func (f *fakeSignalRepo) Save(_ context.Context, _ domain.ExternalSignal) error { return nil }

func (f *fakeSignalRepo) GetLatest(_ context.Context, _ domain.SignalName) (domain.ExternalSignal, error) {
	return domain.ExternalSignal{}, repository.ErrNotFound
}

func (f *fakeSignalRepo) GetForHospital(_ context.Context, _ string, _ time.Time) ([]domain.ExternalSignal, error) {
	return nil, nil
}

func (f *fakeSignalRepo) GetCurrentSignals(_ context.Context, _ string) ([]domain.ExternalSignal, error) {
	return f.current, f.currentErr
}
