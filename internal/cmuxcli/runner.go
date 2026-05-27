package cmuxcli

// Never call: cmux workspace-action --action close-others, cmux workspace-action --action close-above,
// cmux workspace-action --action close-below, cmux close-window, cmux kill.
// (Additive-only invariant per F-NEW; T-039 grep test enforces this comment is the
// ONE allowed match for the forbidden-token scan.)

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// ErrCmuxUnreachable is returned when the cmux binary fails on all 3 retry attempts.
var ErrCmuxUnreachable = errors.New("cmux unreachable after 3 attempts")

// Named retry constants — no magic numbers.
const (
	cmuxRetryCount    = 3
	cmuxRetryInterval = 200 * time.Millisecond
)

// socketBrokenPatterns are stderr substrings that indicate a cmux socket error.
// These are retryable even when the exit code is 0 (should be rare, but defensive).
var socketBrokenPatterns = [][]byte{
	[]byte("connection refused"),
	[]byte("no such file or directory"),
	[]byte("broken pipe"),
}

// runCmux executes `cmux <args...>`, captures stdout and stderr, and retries up to
// cmuxRetryCount times on *exec.ExitError OR when stderr contains a socket-broken
// pattern. Returns ErrCmuxUnreachable after exhaustion.
//
// The retry sleep honors ctx — cancellation during a sleep returns ctx.Err() immediately.
// Argv is always constructed via exec.CommandContext(name, args...) — never via string
// formatting.
func runCmux(ctx context.Context, args ...string) (stdout []byte, stderr []byte, err error) {
	var lastStderr []byte
	for attempt := 0; attempt < cmuxRetryCount; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, lastStderr, ctx.Err()
			case <-time.After(cmuxRetryInterval):
			}
		}

		cmd := exec.CommandContext(ctx, "cmux", args...)
		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf

		runErr := cmd.Run()
		stdout = stdoutBuf.Bytes()
		stderr = stderrBuf.Bytes()
		lastStderr = stderr

		if runErr == nil && !containsAny(stderr, socketBrokenPatterns) {
			return stdout, stderr, nil
		}
	}

	return stdout, lastStderr,
		fmt.Errorf("%w: last stderr: %s", ErrCmuxUnreachable, bytes.TrimSpace(lastStderr))
}

// containsAny returns true when b contains any of the given byte patterns.
func containsAny(b []byte, patterns [][]byte) bool {
	for _, p := range patterns {
		if bytes.Contains(b, p) {
			return true
		}
	}
	return false
}
