package handler

import (
	"context"
	"log/slog"
	"net/http"
)

// Pinger checks DB connectivity for the readiness probe.
//
// Implementers in this codebase:
//   - *db.DB — production wrapper around *pgxpool.Pool. See
//     internal/db/db.go (func (d *DB) Ping). Wired in cmd/api/storage.go.
//   - *fakePinger — test double in internal/handler/fakes_test.go.
type Pinger interface {
	Ping(ctx context.Context) error
}

// LivenessHandler answers GET /health. It carries no external dependencies
// because the liveness probe only confirms the process is alive and serving HTTP.
type LivenessHandler struct {
	log *slog.Logger
}

func NewLivenessHandler(log *slog.Logger) *LivenessHandler {
	return &LivenessHandler{log: log}
}

func (h *LivenessHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.Health)
}

// Health is a liveness probe. Returns 200 as long as the process is running.
func (h *LivenessHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(r.Context(), w, h.log, http.StatusOK, map[string]string{"status": "ok"})
}

// DBReadinessHandler answers GET /ready. Today the only readiness check is a DB
// ping; if more downstream dependencies need to gate traffic later, extend this
// handler rather than reintroducing a generic Pinger to LivenessHandler.
type DBReadinessHandler struct {
	db  Pinger
	log *slog.Logger
}

func NewDBReadinessHandler(db Pinger, log *slog.Logger) *DBReadinessHandler {
	return &DBReadinessHandler{db: db, log: log}
}

func (h *DBReadinessHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ready", h.Ready)
}

// Ready is a readiness probe. Returns 503 when the database is unreachable.
func (h *DBReadinessHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.db.Ping(ctx); err != nil {
		h.log.Error("readiness check", "err", err)
		writeError(ctx, w, h.log, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(ctx, w, h.log, http.StatusOK, map[string]string{"status": "ok"})
}
