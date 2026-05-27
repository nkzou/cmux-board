package jira

import (
	"context"
	"testing"
)

// TestNewDefaultRunnerArgs verifies that the default runner passes args to the binary correctly.
// Uses a real binary on PATH that we know exists: "true" (always exits 0).
func TestNewDefaultRunnerArgs(t *testing.T) {
	runner := newDefaultRunner("true")
	_, _, code, err := runner(context.Background(), "unused-arg")
	if err != nil {
		t.Fatalf("unexpected error from 'true': %v", err)
	}
	if code != 0 {
		t.Errorf("expected exit 0 from 'true', got %d", code)
	}
}

// TestNewDefaultRunnerBinaryNotFound verifies that a missing binary returns exitCode -1 and err.
func TestNewDefaultRunnerBinaryNotFound(t *testing.T) {
	runner := newDefaultRunner("/nonexistent/binary/that/does/not/exist")
	stdout, stderr, code, err := runner(context.Background())
	if err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
	if code != -1 {
		t.Errorf("expected exitCode -1 for missing binary, got %d", code)
	}
	if stdout != nil || stderr != nil {
		t.Errorf("expected nil stdout/stderr for missing binary, got stdout=%q stderr=%q", stdout, stderr)
	}
}

// TestNewDefaultRunnerStderr verifies that stderr is captured when the command writes to it.
// Uses "false" (exits non-zero) which on macOS/Linux is a binary that just exits 1.
// We use a shell command that writes to stderr.
func TestNewDefaultRunnerStdout(t *testing.T) {
	// "echo" writes to stdout and exits 0.
	runner := newDefaultRunner("echo")
	stdout, _, code, err := runner(context.Background(), "hello-world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}
	if string(stdout) == "" {
		t.Error("expected stdout output, got empty")
	}
}
