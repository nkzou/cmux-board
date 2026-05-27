package jira

import (
	"fmt"
	"strings"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// errAuth is returned when acli reports an unauthenticated state.
// It satisfies tracker.AuthError so the sync package can classify failures
// without importing jira-specific types.
type errAuth struct {
	msg string
}

func (e *errAuth) Error() string    { return e.msg }
func (e *errAuth) IsAuthError() bool { return true }

// errFatal is returned for acli errors that are non-auth and non-transition.
type errFatal struct {
	msg    string
	stderr string
}

func (e *errFatal) Error() string {
	if e.stderr != "" {
		return fmt.Sprintf("%s (stderr: %s)", e.msg, e.stderr)
	}
	return e.msg
}

// mapACLIError inspects stdout, stderr, and exit code from an acli invocation
// and returns a typed error. Returns nil if the invocation succeeded with output.
//
// Error classification:
//  1. unauthorized / "use 'acli jira auth login'" → errAuth
//  2. "No allowed transitions found" → tracker.ErrInvalidTransition
//  3. "invalid transition" / "transition not allowed" → tracker.ErrInvalidTransition
//  4. anything else with non-empty stderr or exitCode != 0 → errFatal
//  5. empty stdout with no stderr → errFatal("empty output")
func mapACLIError(stdout, stderr []byte, exitCode int, context string) error {
	stderrStr := strings.TrimSpace(string(stderr))
	stdoutStr := strings.TrimSpace(string(stdout))

	// Check stderr for known patterns first.
	if stderrStr != "" {
		lower := strings.ToLower(stderrStr)
		if strings.Contains(lower, "unauthorized") || strings.Contains(lower, "acli jira auth login") {
			return &errAuth{msg: fmt.Sprintf("%s: unauthorized — run 'acli jira auth login --web'", context)}
		}
		if strings.Contains(lower, "invalid transition") || strings.Contains(lower, "transition not allowed") {
			return tracker.ErrInvalidTransition
		}
	}

	// Non-zero exit with startup failure (ProcessState nil → exitCode -1).
	if exitCode == -1 {
		return &errFatal{msg: fmt.Sprintf("%s: acli not found or failed to start", context), stderr: stderrStr}
	}

	// Non-zero exit with stderr.
	if exitCode != 0 && stderrStr != "" {
		lower := strings.ToLower(stderrStr)
		if strings.Contains(lower, "unauthorized") || strings.Contains(lower, "acli jira auth login") {
			return &errAuth{msg: fmt.Sprintf("%s: unauthorized", context)}
		}
		return &errFatal{msg: fmt.Sprintf("%s: acli exited %d", context, exitCode), stderr: stderrStr}
	}

	// Empty stdout with no other signal.
	if stdoutStr == "" && stderrStr == "" && exitCode == 0 {
		return &errFatal{msg: fmt.Sprintf("%s: empty output from acli", context)}
	}

	return nil
}

// isAuthStderr returns true if the given stderr text signals an auth failure.
func isAuthStderr(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "acli jira auth login")
}
