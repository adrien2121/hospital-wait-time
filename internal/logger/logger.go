package logger

import (
	"log/slog"
	"os"
)

// Build returns a JSON structured logger at the given level.
// Falls back to INFO if logLevel is unrecognised.
func Build(logLevel string) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(logLevel)); err != nil {
		level = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
