// Package logx builds the structured logger used by every service.
package logx

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a JSON logger tagged with the service name.
func New(service string) *slog.Logger {
	level := slog.LevelInfo
	if strings.EqualFold(os.Getenv("LOG_LEVEL"), "debug") {
		level = slog.LevelDebug
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(h).With("service", service)
}

// MaskID masks all but the last 4 characters of an identifier (GSTIN, Udyam, ID number).
// Always use this before logging identifiers.
func MaskID(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return strings.Repeat("*", len(s)-4) + s[len(s)-4:]
}
