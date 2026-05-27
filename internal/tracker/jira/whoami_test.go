package jira

import (
	"context"
	"errors"
	"testing"

	"github.com/nkzou/cmux-board/internal/tracker"
)

func TestWhoAmIHappyPath(t *testing.T) {
	// Simulate authenticated acli auth status output.
	runner := fakeRunner([]byte(
		"✓ Authenticated\n  Site: test.atlassian.net\n  Email: alice@example.com\n  Authentication Type: oauth_global\n",
	), nil, 0, nil)
	a := &JiraAdapter{runner: runner}

	identity, err := a.WhoAmI(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity.Email != "alice@example.com" {
		t.Errorf("Email: got %q, want %q", identity.Email, "alice@example.com")
	}
	if identity.ID != "test.atlassian.net" {
		t.Errorf("ID (site): got %q, want %q", identity.ID, "test.atlassian.net")
	}
}

func TestWhoAmIUnauthenticated_Stderr(t *testing.T) {
	// acli writes auth error to stderr.
	runner := fakeRunner(nil,
		[]byte("✗ Error: unauthorized: use 'acli jira auth login' to authenticate\n"),
		0, nil,
	)
	a := &JiraAdapter{runner: runner}

	_, err := a.WhoAmI(context.Background())
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
	if !tracker.IsAuthError(err) {
		t.Errorf("expected tracker.AuthError, got %T: %v", err, err)
	}
}

func TestWhoAmIUnauthenticated_Stdout(t *testing.T) {
	// Some versions write to stdout instead.
	runner := fakeRunner(
		[]byte("✗ Error: unauthorized: use 'acli jira auth login' to authenticate\n"),
		nil, 0, nil,
	)
	a := &JiraAdapter{runner: runner}

	_, err := a.WhoAmI(context.Background())
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
	if !tracker.IsAuthError(err) {
		t.Errorf("expected tracker.AuthError, got %T: %v", err, err)
	}
}

func TestWhoAmINoAuthenticatedKeyword(t *testing.T) {
	// Output exists but doesn't contain "Authenticated".
	runner := fakeRunner([]byte("some unexpected output\n"), nil, 0, nil)
	a := &JiraAdapter{runner: runner}

	_, err := a.WhoAmI(context.Background())
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
	var ae *errAuth
	if !errors.As(err, &ae) {
		t.Errorf("expected *errAuth, got %T", err)
	}
}

func TestWhoAmIRunnerError(t *testing.T) {
	// Binary not found.
	runner := fakeRunner(nil, nil, -1, errors.New("exec: not found"))
	a := &JiraAdapter{runner: runner}

	_, err := a.WhoAmI(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestWhoAmIArgsPassed(t *testing.T) {
	var capturedArgs []string
	runner := capturingRunner(&capturedArgs,
		[]byte("✓ Authenticated\n  Site: s.atlassian.net\n  Email: u@e.com\n"),
		nil, 0, nil,
	)
	a := &JiraAdapter{runner: runner}
	_, _ = a.WhoAmI(context.Background())

	wantArgs := []string{"jira", "auth", "status"}
	if len(capturedArgs) != len(wantArgs) {
		t.Fatalf("args: got %v, want %v", capturedArgs, wantArgs)
	}
	for i, w := range wantArgs {
		if capturedArgs[i] != w {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], w)
		}
	}
}
