package service

import (
	"context"
	"errors"
	"testing"

	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/repository"
)

func TestHospitalService_GetAll(t *testing.T) {
	t.Run(`
		given a fake hospital repo holding two facilities,
		when the HospitalService asks for the full list,
		then it returns the repo's slice unchanged with every registered hospital`,
		func(t *testing.T) {
			// Given: a repo pre-loaded with two hospitals.
			repo := &fakeHospitalRepo{
				all: []domain.Hospital{
					{ID: "toh-civic", Name: "TOH Civic"},
					{ID: "cheo", Name: "CHEO"},
				},
			}
			svc := NewHospitalService(repo)

			// When: the handler asks for every hospital.
			got, err := svc.GetAll(context.Background())

			// Then: no error, and both hospitals come back in order.
			if err != nil {
				t.Fatalf("GetAll: %v", err)
			}
			if len(got) != 2 {
				t.Fatalf("len = %d, want 2", len(got))
			}
			if got[0].ID != "toh-civic" || got[1].ID != "cheo" {
				t.Fatalf("got IDs %q %q, want toh-civic cheo", got[0].ID, got[1].ID)
			}
		},
	)
}

func TestHospitalService_GetByID_Success(t *testing.T) {
	t.Run(`
		given a fake hospital repo that holds 'toh-civic',
		when the HospitalService is asked for it by ID,
		then it returns the matching Hospital with no error`,
		func(t *testing.T) {
			// Given: a repo that knows toh-civic.
			repo := &fakeHospitalRepo{
				byID: map[string]domain.Hospital{
					"toh-civic": {ID: "toh-civic", Name: "TOH Civic"},
				},
			}
			svc := NewHospitalService(repo)

			// When: the handler asks for the hospital by ID.
			got, err := svc.GetByID(context.Background(), "toh-civic")

			// Then: no error, name matches.
			if err != nil {
				t.Fatalf("GetByID: %v", err)
			}
			if got.Name != "TOH Civic" {
				t.Fatalf("Name = %q, want TOH Civic", got.Name)
			}
		},
	)
}

func TestHospitalService_GetByID_NotFound(t *testing.T) {
	t.Run(`
		given a fake hospital repo with no matching ID,
		when the HospitalService is asked for an unknown hospital,
		then it returns an error chain errors.Is recognises as repository.ErrNotFound`,
		func(t *testing.T) {
			// Given: a repo that returns ErrNotFound for any ID.
			repo := &fakeHospitalRepo{byID: map[string]domain.Hospital{}}
			svc := NewHospitalService(repo)

			// When: the handler asks for a missing hospital.
			_, err := svc.GetByID(context.Background(), "no-such-id")

			// Then: the error wraps repository.ErrNotFound so callers can detect it via errors.Is.
			if !errors.Is(err, repository.ErrNotFound) {
				t.Fatalf("got %v, want errors.Is repository.ErrNotFound", err)
			}
		},
	)
}
