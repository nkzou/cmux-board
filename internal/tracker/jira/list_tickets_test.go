package jira

import (
	"context"
	"strings"
	"testing"
	"time"
)

const boardSearchForProject = `{
  "isLast": true,
  "maxResults": 50,
  "startAt": 0,
  "total": 1,
  "values": [
    {"id": 1, "name": "Test Board", "type": "scrum", "location": "Test Project (TESTPROJ)"}
  ]
}`

const workitemSearchJSON = `[
  {
    "id": "10042",
    "key": "TESTPROJ-42",
    "fields": {
      "summary": "Fix the bug",
      "status": {"id": "3", "name": "In Progress"},
      "assignee": {"accountId": "acc-1", "emailAddress": "user@example.com"},
      "priority": {"name": "High"}
    }
  }
]`

func TestListTicketsHappyPath(t *testing.T) {
	// First call: board search (to resolve project key); second: workitem search.
	runner := sequentialRunner([]runnerResponse{
		{stdout: []byte(boardSearchForProject), exitCode: 0},
		{stdout: []byte(workitemSearchJSON), exitCode: 0},
	})
	a := &JiraAdapter{cfg: Config{Site: "test.atlassian.net"}, runner: runner}

	tickets, err := a.ListTickets(context.Background(), "1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tickets) != 1 {
		t.Fatalf("got %d tickets, want 1", len(tickets))
	}
	tk := tickets[0]
	if tk.ID != "10042" {
		t.Errorf("ID: got %q, want %q", tk.ID, "10042")
	}
	if tk.Key != "TESTPROJ-42" {
		t.Errorf("Key: got %q, want %q", tk.Key, "TESTPROJ-42")
	}
	if tk.Summary != "Fix the bug" {
		t.Errorf("Summary: got %q, want %q", tk.Summary, "Fix the bug")
	}
	if tk.Status != "In Progress" {
		t.Errorf("Status: got %q, want %q", tk.Status, "In Progress")
	}
	if tk.AssigneeID != "acc-1" {
		t.Errorf("AssigneeID: got %q, want %q", tk.AssigneeID, "acc-1")
	}
	if tk.Priority != "High" {
		t.Errorf("Priority: got %q, want %q", tk.Priority, "High")
	}
	// URL must be constructed from site + key.
	wantURL := "https://test.atlassian.net/browse/TESTPROJ-42"
	if tk.URL != wantURL {
		t.Errorf("URL: got %q, want %q", tk.URL, wantURL)
	}
	// Raw must be non-nil.
	if tk.Raw == nil {
		t.Error("Raw must be non-nil")
	}
}

func TestListTicketsSinceFilter(t *testing.T) {
	var allArgs [][]string
	runner := sequentialRunnerWithArgs(&allArgs, []runnerResponse{
		{stdout: []byte(boardSearchForProject), exitCode: 0},
		{stdout: []byte(`[]`), exitCode: 0},
	})
	a := &JiraAdapter{cfg: Config{Site: "test.atlassian.net"}, runner: runner}

	since := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	_, err := a.ListTickets(context.Background(), "1", &since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The second call (workitem search) should have a --jql arg containing 'updated'.
	if len(allArgs) < 2 {
		t.Fatalf("expected 2 calls, got %d", len(allArgs))
	}
	jqlArgs := allArgs[1]
	jqlStr := strings.Join(jqlArgs, " ")
	if !strings.Contains(jqlStr, "updated") {
		t.Errorf("workitem search args missing 'updated' in JQL: %v", jqlArgs)
	}
	if !strings.Contains(jqlStr, "2026-05-26") {
		t.Errorf("workitem search args missing date: %v", jqlArgs)
	}
}

func TestListTicketsAuthError(t *testing.T) {
	runner := fakeRunner(nil,
		[]byte("✗ Error: unauthorized: use 'acli jira auth login' to authenticate"),
		0, nil,
	)
	a := &JiraAdapter{cfg: Config{Site: "test.atlassian.net"}, runner: runner}

	_, err := a.ListTickets(context.Background(), "1", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isErrAuth(err) {
		t.Errorf("expected errAuth, got %T: %v", err, err)
	}
}

func TestListTicketsNullableFields(t *testing.T) {
	const nullableJSON = `[{
		"id": "1", "key": "TESTPROJ-1",
		"fields": {
			"summary": "No assignee or priority",
			"status": {"id": "1", "name": "To Do"},
			"assignee": null,
			"priority": null
		}
	}]`
	runner := sequentialRunner([]runnerResponse{
		{stdout: []byte(boardSearchForProject), exitCode: 0},
		{stdout: []byte(nullableJSON), exitCode: 0},
	})
	a := &JiraAdapter{cfg: Config{Site: "test.atlassian.net"}, runner: runner}

	tickets, err := a.ListTickets(context.Background(), "1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tickets) != 1 {
		t.Fatalf("got %d tickets, want 1", len(tickets))
	}
	if tickets[0].AssigneeID != "" {
		t.Errorf("AssigneeID: got %q, want empty for null assignee", tickets[0].AssigneeID)
	}
	if tickets[0].Priority != "" {
		t.Errorf("Priority: got %q, want empty for null priority", tickets[0].Priority)
	}
}

func TestListTicketsBoardNotFound(t *testing.T) {
	// Board search returns no boards.
	runner := fakeRunner([]byte(`{"isLast":true,"maxResults":50,"startAt":0,"total":0,"values":[]}`), nil, 0, nil)
	a := &JiraAdapter{cfg: Config{Site: "test.atlassian.net"}, runner: runner}

	_, err := a.ListTickets(context.Background(), "999", nil)
	if err == nil {
		t.Fatal("expected error for unknown board, got nil")
	}
}

func TestExtractProjectKey(t *testing.T) {
	tests := []struct {
		location string
		want     string
	}{
		{"IoT Opportunities (IOTOPP)", "IOTOPP"},
		{"Dev Null (DEVNULL)", "DEVNULL"},
		{"My Project Board (ABC123)", "ABC123"},
		{"No key here", ""},
		{"", ""},
		{"App & API Protection (AAP) (APPSEC)", "APPSEC"}, // last key wins
	}
	for _, tt := range tests {
		got := extractProjectKey(tt.location)
		if got != tt.want {
			t.Errorf("extractProjectKey(%q) = %q, want %q", tt.location, got, tt.want)
		}
	}
}
