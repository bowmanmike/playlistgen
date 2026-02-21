package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// New creates a slog.Logger using the provided level and format.
func New(level, format string, out io.Writer) (*slog.Logger, error) {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "json":
		handler = slog.NewJSONHandler(out, opts)
	case "text", "console":
		handler = slog.NewTextHandler(out, opts)
	default:
		return nil, fmt.Errorf("unsupported log format %q", format)
	}

	return slog.New(handler), nil
}

func parseLevel(level string) slog.Leveler {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
