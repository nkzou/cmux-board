package secretsink

import (
	"io"
)

type redactWriter struct {
	inner io.Writer
}

// Writer returns an io.Writer that redacts secrets from every Write call before
// forwarding to inner. Because slog serializes each log record into a single Write call,
// no line-boundary buffering is needed.
func Writer(inner io.Writer) io.Writer {
	return &redactWriter{inner: inner}
}

func (w *redactWriter) Write(p []byte) (int, error) {
	redacted := RedactBytes(p)
	n, err := w.inner.Write(redacted)
	if err != nil {
		return n, err
	}
	// Return original length to avoid "short write" errors in callers
	return len(p), nil
}
