package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/repository"
)

// HospitalService handles read access to hospital metadata for all Ottawa-area facilities.
// One instance covers every hospital. Not one per hospital.
type HospitalService struct {
	repo repository.HospitalRepository
}

func NewHospitalService(repo repository.HospitalRepository) *HospitalService {
	return &HospitalService{repo: repo}
}

func (s *HospitalService) GetAll(ctx context.Context) ([]domain.Hospital, error) {
	hospitals, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all hospitals: %w", err)
	}
	return hospitals, nil
}

func (s *HospitalService) GetByID(ctx context.Context, id string) (domain.Hospital, error) {
	h, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.Hospital{}, fmt.Errorf("hospital %q: %w", id, repository.ErrNotFound)
		}
		return domain.Hospital{}, fmt.Errorf("get hospital %q: %w", id, err)
	}
	return h, nil
}
