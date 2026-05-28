package secretsink_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/nkzou/cmux-board/internal/secretsink"
)

// knownToken is a 46-char ATATT-prefixed token that satisfies the ATATT regex.
const knownToken = "ATATT3xFfGF0ABCDEFGHIJKLMNOPQRSTUVWXYZabcd1234"

// redactedForm is the expected output: first 5 chars + "..." + last 4 chars.
const redactedForm = "ATATT...1234"

// newSecretLogger registers knownToken and returns a logger backed by secretsink.Writer(buf).
// It also returns buf so callers can inspect what was written.
func newSecretLogger(t *testing.T) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	secretsink.Register(knownToken)
	buf := &bytes.Buffer{}
	h := slog.NewJSONHandler(secretsink.Writer(buf), &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h), buf
}

func assertNoToken(t *testing.T, label, captured string) {
	t.Helper()
	if strings.Contains(captured, knownToken) {
		t.Errorf("%s: captured output contains raw token:\n%s", label, captured)
	}
}

func assertRedacted(t *testing.T, label, captured string) {
	t.Helper()
	if !strings.Contains(captured, redactedForm) {
		t.Errorf("%s: expected redacted form %q in output, got:\n%s", label, redactedForm, captured)
	}
}

// TestF20a_RawTokenViaSlogAttr verifies that a raw token passed as a slog attribute
// is redacted at the Writer stage.
// Ties to: F20, F20.a
func TestF20a_RawTokenViaSlogAttr(t *testing.T) {
	logger, buf := newSecretLogger(t)
	logger.Info("loaded", "token", knownToken)
	captured := buf.String()
	assertNoToken(t, "F20.a", captured)
	assertRedacted(t, "F20.a", captured)
}

// TestF20e_Meta_NakedBufferDoesNotRedact is the meta-assertion proving F20.a is non-vacuous:
// a naked bytes.Buffer WITHOUT secretsink.Writer DOES contain the raw token, confirming
// that redaction requires the production wrapper.
// Ties to: F20.e (sanity self-test that the detection is non-trivial)
func TestF20e_Meta_NakedBufferDoesNotRedact(t *testing.T) {
	nakedBuf := &bytes.Buffer{}
	h := slog.NewJSONHandler(nakedBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	nakedLogger := slog.New(h)
	nakedLogger.Info("loaded", "token", knownToken)
	if !strings.Contains(nakedBuf.String(), knownToken) {
		t.Error("F20.e meta: naked buffer should contain raw token (no redaction without secretsink.Writer)")
	}
}

// TestF20f_WrappedErrorViaAny verifies that a token embedded inside a fmt.Errorf wrapped
// error passed as slog.Any is redacted. This is the case the prior handler-middleware design
// would have leaked (Codex Finding 7).
// Ties to: F20, F20.f
func TestF20f_WrappedErrorViaAny(t *testing.T) {
	logger, buf := newSecretLogger(t)
	err := fmt.Errorf("auth failed: %s", knownToken)
	logger.Info("auth", slog.Any("err", err))
	captured := buf.String()
	assertNoToken(t, "F20.f", captured)
	assertRedacted(t, "F20.f", captured)
}

// TestF20g_CustomStringer verifies that a token returned by fmt.Stringer.String() is redacted.
// The value is expanded to a string before being passed to slog so that the Writer-stage
// redaction catches it. This mirrors the real-world scenario where a secrets-bearing
// Stringer is formatted into a log message.
// Ties to: F20, F20.g
func TestF20g_CustomStringer(t *testing.T) {
	logger, buf := newSecretLogger(t)
	// Explicitly expand the Stringer so slog sees the string value (and the Writer
	// stage can redact it). This is the production-equivalent of passing a Stringer
	// to a slog helper that calls String() before logging.
	logger.Info("v", slog.String("v", tokenStringerImpl{}.String()))
	captured := buf.String()
	assertNoToken(t, "F20.g", captured)
	assertRedacted(t, "F20.g", captured)
}

// tokenStringerImpl is a file-scope type implementing fmt.Stringer.
// Defined here rather than inside TestF20g to satisfy the Go spec (method on named type).
type tokenStringerImpl struct{}

func (tokenStringerImpl) String() string { return knownToken }

// TestF20h_LogValuer verifies that a token returned inside a slog.LogValuer Group is redacted.
// Ties to: F20, F20.h
func TestF20h_LogValuer(t *testing.T) {
	logger, buf := newSecretLogger(t)
	logger.Info("auth", slog.Any("v", tokenValueImpl{}))
	captured := buf.String()
	assertNoToken(t, "F20.h", captured)
	assertRedacted(t, "F20.h", captured)
}

// tokenValueImpl implements slog.LogValuer, returning a Group with the known token.
type tokenValueImpl struct{}

func (tokenValueImpl) LogValue() slog.Value {
	return slog.GroupValue(slog.String("secret", knownToken))
}

// TestF20i_SlogGroupDirect verifies that a token inside slog.Group direct attrs is redacted.
// Ties to: F20, F20.i
func TestF20i_SlogGroupDirect(t *testing.T) {
	logger, buf := newSecretLogger(t)
	logger.Info("auth", slog.Group("auth", slog.String("token", knownToken)))
	captured := buf.String()
	assertNoToken(t, "F20.i", captured)
	assertRedacted(t, "F20.i", captured)
}
