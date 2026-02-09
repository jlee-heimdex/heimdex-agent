// Package logging provides structured JSON logging for the Heimdex Agent.
// It uses the standard library log/slog package for structured logging.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger creates a new structured JSON logger with the specified log level.
// Supported levels: debug, info, warn, error
func NewLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: lvl,
		// Add source location for debug level
		AddSource: lvl == slog.LevelDebug,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler)
}

// WithRequestID returns a logger with request_id attribute
func WithRequestID(logger *slog.Logger, requestID string) *slog.Logger {
	return logger.With("request_id", requestID)
}

// WithComponent returns a logger with component attribute
func WithComponent(logger *slog.Logger, component string) *slog.Logger {
	return logger.With("component", component)
}

// WithJobID returns a logger with job_id attribute
func WithJobID(logger *slog.Logger, jobID string) *slog.Logger {
	return logger.With("job_id", jobID)
}

// WithSourceID returns a logger with source_id attribute
func WithSourceID(logger *slog.Logger, sourceID string) *slog.Logger {
	return logger.With("source_id", sourceID)
}

// SanitizeToken masks a token for safe logging.
// Shows first 4 and last 4 characters only.
// Returns "****" for tokens shorter than 8 characters.
func SanitizeToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// SanitizePath masks sensitive parts of a file path.
// Replaces home directory with ~ for privacy.
func SanitizePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
