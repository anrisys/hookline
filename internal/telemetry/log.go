// Package telemetry sets up logging (and, later, tracing and metrics).
package telemetry

import (
	"io"
	"log/slog"
	"strings"
)

// NewLogger returns a JSON slog logger writing to w. Unknown levels fall back
// to info.
func NewLogger(w io.Writer, level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(strings.ToUpper(level))); err != nil {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl}))
}
