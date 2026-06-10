package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/repository"
)

// HospitalQuerier is the subset of service.HospitalService this handler needs.
// Defined here so the handler owns its dependency contract (ISP).
type HospitalQuerier interface {
	GetAll(ctx context.Context) ([]domain.Hospital, error)
	GetByID(ctx context.Context, id string) (domain.Hospital, error)
}

type HospitalHandler struct {
	svc HospitalQuerier
	log *slog.Logger
}

func NewHospitalHandler(svc HospitalQuerier, log *slog.Logger) *HospitalHandler {
	return &HospitalHandler{svc: svc, log: log}
}

func (h *HospitalHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/hospitals", h.ListHospitals)
	mux.HandleFunc("GET /api/v1/hospitals/{id}", h.GetHospital)
}

func (h *HospitalHandler) ListHospitals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hospitals, err := h.svc.GetAll(ctx)
	if err != nil {
		h.log.Error("list hospitals", "err", err)
		writeError(ctx, w, h.log, http.StatusInternalServerError, "internal error")
		return
	}
	resp := make([]hospitalResponse, len(hospitals))
	for i, hosp := range hospitals {
		resp[i] = toHospitalResponse(hosp)
	}
	writeJSON(ctx, w, h.log, http.StatusOK, resp)
}

func (h *HospitalHandler) GetHospital(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	hosp, err := h.svc.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(ctx, w, h.log, http.StatusNotFound, "hospital not found")
			return
		}
		h.log.Error("get hospital", "id", id, "err", err)
		writeError(ctx, w, h.log, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(ctx, w, h.log, http.StatusOK, toHospitalResponse(hosp))
}

// hospitalResponse is the JSON shape for a hospital — snake_case keys, string FacilityType.
type hospitalResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Address      string `json:"address"`
	FacilityType string `json:"facility_type"`
	SourceURL    string `json:"source_url"`
	Active       bool   `json:"active"`
}

func toHospitalResponse(h domain.Hospital) hospitalResponse {
	return hospitalResponse{
		ID:           h.ID,
		Name:         h.Name,
		Address:      h.Address,
		FacilityType: string(h.FacilityType),
		SourceURL:    h.SourceURL,
		Active:       h.Active,
	}
}
