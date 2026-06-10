package repository

import (
	"context"

	"github.com/adrien2121/GoProject/internal/domain"
)

// HospitalRepository defines data access for Hospital entities.
type HospitalRepository interface {
	GetAll(ctx context.Context) ([]domain.Hospital, error)
	GetByID(ctx context.Context, id string) (domain.Hospital, error)
	Upsert(ctx context.Context, h domain.Hospital) error
}
