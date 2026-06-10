package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// writeJSON sets Content-Type, writes status, encodes v as JSON.
// On encode failure (NaN, unsupported type, etc.) status is already on the wire
// so we can't change it. Log so the failure isn't silent in prod.
func writeJSON(ctx context.Context, w http.ResponseWriter, log *slog.Logger, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.ErrorContext(ctx, "encode response", "err", err, "status", status)
	}
}

// writeError wraps msg in {"error": "..."} and calls writeJSON.
func writeError(ctx context.Context, w http.ResponseWriter, log *slog.Logger, status int, msg string) {
	writeJSON(ctx, w, log, status, map[string]string{"error": msg})
}
