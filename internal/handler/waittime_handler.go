package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/adrien2121/GoProject/internal/domain"
	"github.com/adrien2121/GoProject/internal/repository"
)

// WaitTimeQuerier is the subset of service.WaitTimeService this handler needs.
type WaitTimeQuerier interface {
	GetAllCurrentStatus(ctx context.Context) ([]domain.HospitalStatus, error)
	GetHospitalStatus(ctx context.Context, id string) (domain.HospitalStatus, error)
	GetHistory(ctx context.Context, hospitalID string, from, to time.Time) ([]domain.WaitTimeSnapshot, error)
	GetBestTimeToVisit(ctx context.Context, hospitalID string) (domain.BestTimeSlot, error)
}

type WaitTimeHandler struct {
	svc WaitTimeQuerier
	log *slog.Logger
}

func NewWaitTimeHandler(svc WaitTimeQuerier, log *slog.Logger) *WaitTimeHandler {
	return &WaitTimeHandler{svc: svc, log: log}
}

func (h *WaitTimeHandler) RegisterRoutes(mux *http.ServeMux) {
	// /current and /summary serve the same payload; one handler, two routes.
	mux.HandleFunc("GET /api/v1/wait-times/current", h.GetAllCurrent)
	mux.HandleFunc("GET /api/v1/wait-times/summary", h.GetAllCurrent)
	mux.HandleFunc("GET /api/v1/hospitals/{id}/wait-times/current", h.GetHospitalCurrent)
	mux.HandleFunc("GET /api/v1/hospitals/{id}/wait-times/history", h.GetHistory)
	mux.HandleFunc("GET /api/v1/hospitals/{id}/wait-times/best-time", h.GetBestTime)
}

func (h *WaitTimeHandler) GetAllCurrent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	statuses, err := h.svc.GetAllCurrentStatus(ctx)
	if err != nil {
		h.log.Error("get all current", "err", err)
		writeError(ctx, w, h.log, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(ctx, w, h.log, http.StatusOK, toStatusResponses(statuses))
}

func (h *WaitTimeHandler) GetHospitalCurrent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	status, err := h.svc.GetHospitalStatus(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(ctx, w, h.log, http.StatusNotFound, "hospital not found")
			return
		}
		h.log.Error("get hospital current", "id", id, "err", err)
		writeError(ctx, w, h.log, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(ctx, w, h.log, http.StatusOK, toStatusResponse(status))
}

func (h *WaitTimeHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	q := r.URL.Query()

	from, err := time.Parse(time.RFC3339, q.Get("from"))
	if err != nil {
		writeError(ctx, w, h.log, http.StatusBadRequest, "from: must be RFC3339 (e.g. 2026-06-01T00:00:00Z)")
		return
	}
	to, err := time.Parse(time.RFC3339, q.Get("to"))
	if err != nil {
		writeError(ctx, w, h.log, http.StatusBadRequest, "to: must be RFC3339 (e.g. 2026-06-07T00:00:00Z)")
		return
	}
	if !from.Before(to) {
		writeError(ctx, w, h.log, http.StatusBadRequest, "from must be before to")
		return
	}

	snaps, err := h.svc.GetHistory(ctx, id, from, to)
	if err != nil {
		h.log.Error("get history", "id", id, "err", err)
		writeError(ctx, w, h.log, http.StatusInternalServerError, "internal error")
		return
	}
	resp := make([]snapshotResponse, len(snaps))
	for i, s := range snaps {
		resp[i] = toSnapshotResponse(s)
	}
	writeJSON(ctx, w, h.log, http.StatusOK, resp)
}

func (h *WaitTimeHandler) GetBestTime(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	best, err := h.svc.GetBestTimeToVisit(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(ctx, w, h.log, http.StatusNotFound, "hospital not found or insufficient data")
			return
		}
		h.log.Error("get best time", "id", id, "err", err)
		writeError(ctx, w, h.log, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(ctx, w, h.log, http.StatusOK, bestTimeResponse{
		HospitalID:     best.HospitalID,
		DayOfWeek:      best.DayOfWeek,
		Hour:           best.Hour,
		AvgWaitMinutes: best.AvgWaitMinutes,
	})
}
