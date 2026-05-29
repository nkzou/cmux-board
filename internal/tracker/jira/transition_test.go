package jira

import (
	"context"
	"errors"
	"testing"

	"github.com/nkzou/cmux-board/internal/tracker"
)

const transitionSuccessJSON = `{
  "results": [{"status":"SUCCESS","message":"Work item PROJ-1 has been successfully transitioned to In Progress","id":"PROJ-1"}],
  "totalCount": 1,
  "successCount": 1
}`

const transitionFailureInvalidJSON = `{
  "results": [{"status":"FAILURE","message":"No allowed transitions found for given status","id":"PROJ-1"}],
  "totalCount": 1,
  "successCount": 0
}`

const transitionFailureNotFoundJSON = `{
  "results": [{"status":"FAILURE","message":"Issue does not exist or you do not have permission to see it.","id":"PROJ-1"}],
  "totalCount": 1,
  "successCount": 0
}`

func workitemViewStatus(name string) []byte {
	return []byte(`{"id":"10001","key":"PROJ-1","fields":{"status":{"id":"1","name":"` + name + `"}}}`)
}

func TestTransitionStatusHappyPath(t *testing.T) {
	var allArgs [][]string
	runner := sequentialRunnerWithArgs(&allArgs, []runnerResponse{
		{stdout: workitemViewStatus("To Do"), exitCode: 0},      // fetchCurrentStatus
		{stdout: []byte(transitionSuccessJSON), exitCode: 0},    // transition
	})
	a := &JiraAdapter{runner: runner}

	err := a.TransitionStatus(context.Background(), "PROJ-1", "To Do", "In Progress")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(allArgs) != 2 {
		t.Errorf("expected 2 acli calls, got %d", len(allArgs))
	}
	// First call: workitem view
	if len(allArgs[0]) < 3 || allArgs[0][2] != "view" {
		t.Errorf("first call: expected workitem view, got %v", allArgs[0])
	}
	// Second call: workitem transition with --yes
	found := false
	for _, arg := range allArgs[1] {
		if arg == "--yes" {
			found = true
		}
	}
	if !found {
		t.Errorf("transition call: --yes not found in args: %v", allArgs[1])
	}
}

func TestTransitionStatusOCCMismatch(t *testing.T) {
	var allArgs [][]string
	runner := sequentialRunnerWithArgs(&allArgs, []runnerResponse{
		// Current status is "In Progress" but caller expected "To Do"
		{stdout: workitemViewStatus("In Progress"), exitCode: 0},
	})
	a := &JiraAdapter{runner: runner}

	err := a.TransitionStatus(context.Background(), "PROJ-1", "To Do", "Done")
	if err == nil {
		t.Fatal("expected ConflictError, got nil")
	}
	if !errors.Is(err, tracker.ErrConflict) {
		t.Errorf("expected tracker.ErrConflict, got %T: %v", err, err)
	}
	// Only 1 call (the view); no transition call.
	if len(allArgs) != 1 {
		t.Errorf("expected exactly 1 acli call on OCC mismatch, got %d", len(allArgs))
	}
	// ConflictError should carry server status.
	var ce tracker.ConflictError
	if errors.As(err, &ce) {
		if ce.ServerStatus != "In Progress" {
			t.Errorf("ConflictError.ServerStatus: got %q, want %q", ce.ServerStatus, "In Progress")
		}
	}
}

func TestTransitionStatusCaseInsensitiveMatch(t *testing.T) {
	runner := sequentialRunner([]runnerResponse{
		{stdout: workitemViewStatus("to do"), exitCode: 0},
		{stdout: []byte(transitionSuccessJSON), exitCode: 0},
	})
	a := &JiraAdapter{runner: runner}

	// expectedFromStatus "To Do" vs server "to do" — should match case-insensitively.
	err := a.TransitionStatus(context.Background(), "PROJ-1", "To Do", "In Progress")
	if err != nil {
		t.Fatalf("expected case-insensitive match to succeed, got: %v", err)
	}
}

func TestTransitionStatusInvalidTransition(t *testing.T) {
	runner := sequentialRunner([]runnerResponse{
		{stdout: workitemViewStatus("To Do"), exitCode: 0},
		{stdout: []byte(transitionFailureInvalidJSON), exitCode: 0},
	})
	a := &JiraAdapter{runner: runner}

	err := a.TransitionStatus(context.Background(), "PROJ-1", "To Do", "Nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, tracker.ErrInvalidTransition) {
		t.Errorf("expected tracker.ErrInvalidTransition, got %T: %v", err, err)
	}
}

func TestTransitionStatusNotFound(t *testing.T) {
	runner := sequentialRunner([]runnerResponse{
		{stdout: workitemViewStatus("To Do"), exitCode: 0},
		{stdout: []byte(transitionFailureNotFoundJSON), exitCode: 0},
	})
	a := &JiraAdapter{runner: runner}

	err := a.TransitionStatus(context.Background(), "PROJ-1", "To Do", "Done")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var fe *errFatal
	if !errors.As(err, &fe) {
		t.Errorf("expected *errFatal, got %T: %v", err, err)
	}
}

func TestTransitionStatusAuthError_View(t *testing.T) {
	runner := fakeRunner(nil,
		[]byte("✗ Error: unauthorized: use 'acli jira auth login' to authenticate"),
		0, nil,
	)
	a := &JiraAdapter{runner: runner}

	err := a.TransitionStatus(context.Background(), "PROJ-1", "To Do", "Done")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isErrAuth(err) {
		t.Errorf("expected errAuth, got %T: %v", err, err)
	}
}

func TestTransitionStatusAuthError_Transition(t *testing.T) {
	runner := sequentialRunner([]runnerResponse{
		{stdout: workitemViewStatus("To Do"), exitCode: 0},
		{stderr: []byte("✗ Error: unauthorized: use 'acli jira auth login' to authenticate"), exitCode: 0},
	})
	a := &JiraAdapter{runner: runner}

	err := a.TransitionStatus(context.Background(), "PROJ-1", "To Do", "Done")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isErrAuth(err) {
		t.Errorf("expected errAuth, got %T: %v", err, err)
	}
}
