package runtime

import (
	"io"
	"log/slog"
	"os"

	"github.com/nkzou/cmux-board/internal/secretsink"
)

// NewLogger builds a structured slog.Logger that writes to stderr through the
// secretsink.Writer redaction layer. Call secretsink.Register for the API token
// before calling this function so the writer can redact it from the first log line.
//
// The returned logger is also installed as the slog default via slog.SetDefault.
func NewLogger(level slog.Level) *slog.Logger {
	return newLoggerWithWriter(level, os.Stderr)
}

// NewLoggerWithWriter builds a logger writing to w through the secretsink.Writer
// redaction layer. Used in tests to capture output via a bytes.Buffer and in
// dock_cmd.go for log capture during testing.
func NewLoggerWithWriter(level slog.Level, w io.Writer) *slog.Logger {
	return newLoggerWithWriter(level, w)
}

// newLoggerWithWriter is the internal implementation.
func newLoggerWithWriter(level slog.Level, w io.Writer) *slog.Logger {
	// Codex Finding 7: construct the handler around secretsink.Writer(w), never
	// directly around w. Redaction happens at the io.Writer stage so it applies
	// regardless of attribute shape (string, error, Stringer, Group, etc.).
	redacted := secretsink.Writer(w)
	handler := slog.NewTextHandler(redacted, &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
