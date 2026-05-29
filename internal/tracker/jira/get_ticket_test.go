package jira

import (
	"context"
	"errors"
	"testing"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// workitemViewFullJSON includes all fields that GetTicket maps: id, key, fields.summary,
// fields.status, fields.assignee, fields.priority.
const workitemViewFullJSON = `{
  "id": "10001",
  "key": "PROJ-42",
  "fields": {
    "summary": "Fix the thing",
    "issuetype": {"name": "Task"},
    "status": {"id": "3", "name": "In Progress"},
    "assignee": {"accountId": "user-abc-123", "emailAddress": "dev@example.com"},
    "priority": {"name": "High"}
  }
}`

func TestGetTicket_Success(t *testing.T) {
	var capturedArgs []string
	runner := capturingRunner(&capturedArgs, []byte(workitemViewFullJSON), nil, 0, nil)
	a := &JiraAdapter{runner: runner, cfg: Config{Site: "myjira.atlassian.net"}}

	got, err := a.GetTicket(context.Background(), "PROJ-42")
	if err != nil {
		t.Fatalf("GetTicket: unexpected error: %v", err)
	}

	if got.Key != "PROJ-42" {
		t.Errorf("Key: got %q, want %q", got.Key, "PROJ-42")
	}
	if got.Summary != "Fix the thing" {
		t.Errorf("Summary: got %q, want %q", got.Summary, "Fix the thing")
	}
	if got.Status != "In Progress" {
		t.Errorf("Status: got %q, want %q", got.Status, "In Progress")
	}
	if got.URL != "https://myjira.atlassian.net/browse/PROJ-42" {
		t.Errorf("URL: got %q, want %q", got.URL, "https://myjira.atlassian.net/browse/PROJ-42")
	}
	if got.Priority != "High" {
		t.Errorf("Priority: got %q, want %q", got.Priority, "High")
	}
	if got.IssueType != "Task" {
		t.Errorf("IssueType: got %q, want %q", got.IssueType, "Task")
	}
	if got.AssigneeID != "user-abc-123" {
		t.Errorf("AssigneeID: got %q, want %q", got.AssigneeID, "user-abc-123")
	}
	if got.AssigneeEmail != "dev@example.com" {
		t.Errorf("AssigneeEmail: got %q, want %q", got.AssigneeEmail, "dev@example.com")
	}

	// Verify the correct acli sub-command was used.
	found := false
	for _, arg := range capturedArgs {
		if arg == "view" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'view' in acli args, got: %v", capturedArgs)
	}
}

func TestGetTicket_NotFound(t *testing.T) {
	// acli exits non-zero with a "does not exist" message in stderr.
	runner := fakeRunner(
		nil,
		[]byte("issue PROJ-99 does not exist or you do not have permission to see it"),
		1,
		nil,
	)
	a := &JiraAdapter{runner: runner, cfg: Config{Site: "myjira.atlassian.net"}}

	_, err := a.GetTicket(context.Background(), "PROJ-99")
	if err == nil {
		t.Fatal("GetTicket: expected error for not-found ticket, got nil")
	}
}

func TestGetTicket_AuthError(t *testing.T) {
	// acli exits non-zero with an auth error in stderr.
	runner := fakeRunner(
		nil,
		[]byte("unauthorized — run 'acli jira auth login --web'"),
		1,
		nil,
	)
	a := &JiraAdapter{runner: runner, cfg: Config{Site: "myjira.atlassian.net"}}

	_, err := a.GetTicket(context.Background(), "PROJ-1")
	if err == nil {
		t.Fatal("GetTicket: expected auth error, got nil")
	}
	if !tracker.IsAuthError(err) {
		t.Errorf("GetTicket: expected auth error, got: %v (type %T)", err, err)
	}
}

func TestGetTicket_RunnerError(t *testing.T) {
	// Simulate acli not found or other runner-level failure.
	runner := fakeRunner(nil, nil, -1, errors.New("exec: not found"))
	a := &JiraAdapter{runner: runner, cfg: Config{Site: "myjira.atlassian.net"}}

	_, err := a.GetTicket(context.Background(), "PROJ-1")
	if err == nil {
		t.Fatal("GetTicket: expected error from runner failure, got nil")
	}
}
