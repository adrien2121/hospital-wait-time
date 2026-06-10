package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/repository"
)

func TestGetAllCurrentStatus_SkipsInactiveHospitals(t *testing.T) {
	t.Run(`
		given two hospitals in the repo where one is marked Active=false,
		when the API asks for every current status,
		then only the active hospital appears in the response and closed facilities are excluded`,
		func(t *testing.T) {
			// Given: one active and one inactive hospital, both with snapshots in the repo.
			now := time.Now()
			hospRepo := &fakeHospitalRepo{
				all: []domain.Hospital{
					{ID: "active", Active: true},
					{ID: "closed", Active: false},
				},
			}
			waitRepo := &fakeWaitTimeRepo{
				allLatest: []domain.WaitTimeSnapshot{
					{HospitalID: "active", WaitMinutes: 30, ScrapedAt: now},
					{HospitalID: "closed", WaitMinutes: 99, ScrapedAt: now},
				},
				history: nil,
			}
			signalRepo := &fakeSignalRepo{}
			svc := NewWaitTimeService(waitRepo, hospRepo, signalRepo)

			// When: the API asks for the full current status list.
			got, err := svc.GetAllCurrentStatus(context.Background())

			// Then: only the active hospital comes back.
			if err != nil {
				t.Fatalf("GetAllCurrentStatus: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("len = %d, want 1", len(got))
			}
			if got[0].Hospital.ID != "active" {
				t.Fatalf("ID = %q, want active", got[0].Hospital.ID)
			}
		},
	)
}

func TestGetHospitalStatus_NoSnapshotYet(t *testing.T) {
	t.Run(`
		given a hospital that exists but has never been scraped,
		when the API asks for its current status,
		then the response carries Latest=nil and Trend=Stable so the caller can render a 'no data yet' placeholder`,
		func(t *testing.T) {
			// Given: the hospital exists but the wait repo returns ErrNotFound for its latest snapshot.
			hospRepo := &fakeHospitalRepo{
				byID: map[string]domain.Hospital{"new-hosp": {ID: "new-hosp", Active: true}},
			}
			waitRepo := &fakeWaitTimeRepo{latestErr: repository.ErrNotFound}
			signalRepo := &fakeSignalRepo{}
			svc := NewWaitTimeService(waitRepo, hospRepo, signalRepo)

			// When: the API asks for that hospital's current status.
			got, err := svc.GetHospitalStatus(context.Background(), "new-hosp")

			// Then: Latest is nil, Trend defaults to Stable.
			if err != nil {
				t.Fatalf("GetHospitalStatus: %v", err)
			}
			if got.Latest != nil {
				t.Fatalf("Latest = %+v, want nil", got.Latest)
			}
			if got.Trend != domain.TrendStable {
				t.Fatalf("Trend = %q, want %q", got.Trend, domain.TrendStable)
			}
		},
	)
}

func TestGetHospitalStatus_UnknownHospital(t *testing.T) {
	t.Run(`
		given the hospital repo has no row for the requested ID,
		when the API asks for that hospital's current status,
		then the error chain wraps repository.ErrNotFound and the handler can map it to a 404`,
		func(t *testing.T) {
			// Given: empty hospital repo.
			hospRepo := &fakeHospitalRepo{byID: map[string]domain.Hospital{}}
			waitRepo := &fakeWaitTimeRepo{}
			signalRepo := &fakeSignalRepo{}
			svc := NewWaitTimeService(waitRepo, hospRepo, signalRepo)

			// When: the API asks for an unknown hospital.
			_, err := svc.GetHospitalStatus(context.Background(), "no-such-id")

			// Then: the wrapper preserves the sentinel for errors.Is.
			if !errors.Is(err, repository.ErrNotFound) {
				t.Fatalf("got %v, want errors.Is repository.ErrNotFound", err)
			}
		},
	)
}

func TestGetHistory_RejectsInvertedWindow(t *testing.T) {
	t.Run(`
		given a query window where from is after to (or equal to),
		when the API asks GetHistory to fetch snapshots,
		then the service refuses with an error before any SQL runs`,
		func(t *testing.T) {
			// Given: an inverted window — from after to.
			hospRepo := &fakeHospitalRepo{}
			waitRepo := &fakeWaitTimeRepo{}
			signalRepo := &fakeSignalRepo{}
			svc := NewWaitTimeService(waitRepo, hospRepo, signalRepo)

			from := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
			to := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

			// When: the handler asks for the history.
			_, err := svc.GetHistory(context.Background(), "any", from, to)

			// Then: the service rejects the request.
			if err == nil {
				t.Fatal("expected error for inverted window")
			}
		},
	)
}

func TestGetBestTimeToVisit_PicksTheLowestAverage(t *testing.T) {
	t.Run(`
		given the wait repo returns three hourly averages for a hospital,
		when the API asks for the best time to visit,
		then the response carries the row with the lowest AvgWaitMinutes`,
		func(t *testing.T) {
			// Given: three hourly averages with 20 as the smallest.
			hospRepo := &fakeHospitalRepo{}
			waitRepo := &fakeWaitTimeRepo{
				hourly: []repository.HourlyAverage{
					{HospitalID: "h", DayOfWeek: 1, Hour: 9, AvgWaitMinutes: 60},
					{HospitalID: "h", DayOfWeek: 2, Hour: 4, AvgWaitMinutes: 20},
					{HospitalID: "h", DayOfWeek: 3, Hour: 14, AvgWaitMinutes: 45},
				},
			}
			signalRepo := &fakeSignalRepo{}
			svc := NewWaitTimeService(waitRepo, hospRepo, signalRepo)

			// When: the API asks for the best time.
			got, err := svc.GetBestTimeToVisit(context.Background(), "h")

			// Then: the response picks the row with the smallest average.
			if err != nil {
				t.Fatalf("GetBestTimeToVisit: %v", err)
			}
			if got.AvgWaitMinutes != 20 {
				t.Fatalf("AvgWaitMinutes = %v, want 20", got.AvgWaitMinutes)
			}
			if got.Hour != 4 || got.DayOfWeek != "Tuesday" {
				t.Fatalf("got Hour=%d DoW=%q, want Hour=4 DoW=Tuesday", got.Hour, got.DayOfWeek)
			}
		},
	)
}

func TestGetBestTimeToVisit_NoData(t *testing.T) {
	t.Run(`
		given the wait repo returns zero averages (hospital just added or no history yet),
		when the API asks for the best time,
		then the error chain wraps repository.ErrNotFound and the handler can map it to a 404 with an 'insufficient data' message`,
		func(t *testing.T) {
			// Given: the repo returns an empty slice (no historical data yet).
			waitRepo := &fakeWaitTimeRepo{hourly: nil}
			svc := NewWaitTimeService(waitRepo, &fakeHospitalRepo{}, &fakeSignalRepo{})

			// When: the API asks for the best time.
			_, err := svc.GetBestTimeToVisit(context.Background(), "h")

			// Then: the error wraps ErrNotFound.
			if !errors.Is(err, repository.ErrNotFound) {
				t.Fatalf("got %v, want errors.Is repository.ErrNotFound", err)
			}
		},
	)
}
