package jira

import (
	"context"
	"os/exec"
)

// Runner executes acli with the given arguments and returns stdout, stderr, exit code, and any
// process-level error (e.g., binary not found). It does NOT return a non-nil error for non-zero
// exit codes — callers must inspect exitCode and stderr themselves.
type Runner func(ctx context.Context, args ...string) (stdout, stderr []byte, exitCode int, err error)

// newDefaultRunner returns a Runner that shells out to the acli binary at the given path.
func newDefaultRunner(acliPath string) Runner {
	return func(ctx context.Context, args ...string) ([]byte, []byte, int, error) {
		cmd := exec.CommandContext(ctx, acliPath, args...)
		var outBuf, errBuf errorCapture
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf

		runErr := cmd.Run()
		code := 0
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}

		// If the process didn't start at all (binary not found, permission denied),
		// runErr is non-nil but ProcessState is nil (ExitCode returns -1).
		// Distinguish: non-nil error with no ProcessState = startup failure.
		if runErr != nil && cmd.ProcessState == nil {
			return nil, nil, -1, runErr
		}

		return outBuf.b, errBuf.b, code, nil
	}
}

// errorCapture is a simple byte-buffer writer used for stdout/stderr capture.
type errorCapture struct{ b []byte }

func (e *errorCapture) Write(p []byte) (int, error) {
	e.b = append(e.b, p...)
	return len(p), nil
}
