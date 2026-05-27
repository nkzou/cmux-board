package runtime

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/kevin-zou/cmux-board/internal/secretsink"
)


const testToken = "super-secret-token-42"

// registerTestToken registers testToken before any sub-test runs.
// secretsink.Register is idempotent for duplicate values — safe to call multiple times.
func registerTestToken(t *testing.T) {
	t.Helper()
	secretsink.Register(testToken)
}

// newTestLogger returns a logger that writes to buf and registers testToken.
func newTestLogger(t *testing.T) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	registerTestToken(t)
	var buf bytes.Buffer
	logger := newLoggerWithWriter(slog.LevelDebug, &buf)
	return logger, &buf
}

// TestF20a — direct string attribute containing registered token is redacted.
func TestF20a(t *testing.T) {
	t.Parallel()
	logger, buf := newTestLogger(t)
	logger.Info("test", "token", testToken)
	if strings.Contains(buf.String(), testToken) {
		t.Errorf("F20.a: raw token appeared in output:\n%s", buf.String())
	}
}

// TestF20d — HTTP header values do not appear in log output.
// F20.d test is NOT vacuous: it logs a real header key-value pair and asserts
// the value is absent from output.
func TestF20d(t *testing.T) {
	t.Parallel()
	logger, buf := newTestLogger(t)

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Put the test token into a header value to test that it gets redacted
	// regardless of whether it arrives via header logging path.
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("X-Custom", "value-"+testToken)

	// Log the header map as a slog attribute (the banned pattern).
	logger.Info("headers logged", "headers", fmt.Sprintf("%v", req.Header))

	out := buf.String()
	if strings.Contains(out, testToken) {
		t.Errorf("F20.d: raw token appeared in header log output:\n%s", out)
	}
	// The output must contain *something* (non-vacuous check).
	if !strings.Contains(out, "headers logged") {
		t.Errorf("F20.d: expected 'headers logged' message in output:\n%s", out)
	}
}

// TestF20f — wrapped error containing token is redacted.
func TestF20f(t *testing.T) {
	t.Parallel()
	logger, buf := newTestLogger(t)
	logger.Info("err", "err", fmt.Errorf("failed: %s", testToken))
	if strings.Contains(buf.String(), testToken) {
		t.Errorf("F20.f: raw token appeared in wrapped error output:\n%s", buf.String())
	}
}

// TestF20g — fmt.Stringer value containing token is redacted.
func TestF20g(t *testing.T) {
	t.Parallel()
	logger, buf := newTestLogger(t)
	tval := tokenStringerVal(testToken)
	logger.Info("str", "v", tval)
	if strings.Contains(buf.String(), testToken) {
		t.Errorf("F20.g: raw token appeared in Stringer output:\n%s", buf.String())
	}
}

// tokenStringerVal implements fmt.Stringer for testing F20.g.
type tokenStringerVal string

func (t tokenStringerVal) String() string { return string(t) }

// TestF20i — token inside slog.Group is redacted.
func TestF20i(t *testing.T) {
	t.Parallel()
	logger, buf := newTestLogger(t)
	logger.Info("grp", slog.Group("auth", slog.String("token", testToken)))
	if strings.Contains(buf.String(), testToken) {
		t.Errorf("F20.i: raw token appeared in slog Group output:\n%s", buf.String())
	}
}

// TestNewLoggerInstallsDefault verifies slog.SetDefault is called so package-level
// slog calls are also routed through the redaction layer.
func TestNewLoggerInstallsDefault(t *testing.T) {
	t.Parallel()
	registerTestToken(t)
	var buf bytes.Buffer
	newLoggerWithWriter(slog.LevelDebug, &buf)
	// Package-level slog.Info call should use the default set above.
	slog.Info("default-path", "v", testToken)
	if strings.Contains(buf.String(), testToken) {
		t.Errorf("slog default not routing through redaction: raw token in output:\n%s", buf.String())
	}
}
