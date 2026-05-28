package jira

import (
	"context"
	"strings"
	"testing"
	"time"
)

// boardGetForProject is the JSON shape returned by `acli jira board get --id 1 --json`
// when resolveProjectKey looks up the board to extract its project key.
const boardGetForProject = `{
  "id": 1,
  "name": "Test Board",
  "type": "scrum",
  "location": "Test Project (TESTPROJ)",
  "link": "https://test.atlassian.net/jira/software/projects/TESTPROJ/boards/1"
}`

const workitemSearchJSON = `[
  {
    "id": "10042",
    "key": "TESTPROJ-42",
    "fields": {
      "summary": "Fix the bug",
      "issuetype": {"name": "Bug"},
      "status": {"id": "3", "name": "In Progress"},
      "assignee": {"accountId": "acc-1", "emailAddress": "user@example.com"},
      "priority": {"name": "High"}
    }
  }
]`

func TestListTicketsHappyPath(t *testing.T) {
	// First call: board get (to resolve project key); second: workitem search.
	runner := sequentialRunner([]runnerResponse{
		{stdout: []byte(boardGetForProject), exitCode: 0},
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
	if tk.IssueType != "Bug" {
		t.Errorf("IssueType: got %q, want %q", tk.IssueType, "Bug")
	}
	if tk.AssigneeID != "acc-1" {
		t.Errorf("AssigneeID: got %q, want %q", tk.AssigneeID, "acc-1")
	}
	if tk.AssigneeEmail != "user@example.com" {
		t.Errorf("AssigneeEmail: got %q, want %q", tk.AssigneeEmail, "user@example.com")
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
		{stdout: []byte(boardGetForProject), exitCode: 0},
		{stdout: []byte(`[]`), exitCode: 0},
	})
	a := &JiraAdapter{cfg: Config{Site: "test.atlassian.net"}, runner: runner}

	// since is within the 3-month window, so it should be the binding cutoff.
	since := time.Now().Add(-24 * time.Hour)
	_, err := a.ListTickets(context.Background(), "1", &since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(allArgs) < 2 {
		t.Fatalf("expected 2 calls, got %d", len(allArgs))
	}
	jqlStr := strings.Join(allArgs[1], " ")
	if !strings.Contains(jqlStr, "updated") {
		t.Errorf("workitem search args missing 'updated' in JQL: %v", allArgs[1])
	}
	wantDate := since.UTC().Format("2006-01-02 15:04")
	if !strings.Contains(jqlStr, wantDate) {
		t.Errorf("workitem search args missing since date %q: %v", wantDate, allArgs[1])
	}
}

func TestListTicketsThreeMonthFloorWhenSinceNil(t *testing.T) {
	var allArgs [][]string
	runner := sequentialRunnerWithArgs(&allArgs, []runnerResponse{
		{stdout: []byte(boardGetForProject), exitCode: 0},
		{stdout: []byte(`[]`), exitCode: 0},
	})
	a := &JiraAdapter{cfg: Config{Site: "test.atlassian.net"}, runner: runner}

	_, err := a.ListTickets(context.Background(), "1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	jqlStr := strings.Join(allArgs[1], " ")
	if !strings.Contains(jqlStr, "updated >=") {
		t.Errorf("expected 'updated >=' in JQL when since is nil, got: %v", allArgs[1])
	}
	// Floor date should be ~3 months ago. Check the year matches roughly.
	floor := time.Now().Add(-listTicketsWindow).UTC().Format("2006-01-02")
	if !strings.Contains(jqlStr, floor) {
		t.Errorf("expected 3-month floor date %q in JQL, got: %v", floor, allArgs[1])
	}
}

func TestListTicketsFiltersByCurrentUser(t *testing.T) {
	var allArgs [][]string
	runner := sequentialRunnerWithArgs(&allArgs, []runnerResponse{
		{stdout: []byte(boardGetForProject), exitCode: 0},
		{stdout: []byte(`[]`), exitCode: 0},
	})
	a := &JiraAdapter{cfg: Config{Site: "test.atlassian.net"}, runner: runner}

	if _, err := a.ListTickets(context.Background(), "1", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	jqlStr := strings.Join(allArgs[1], " ")
	if !strings.Contains(jqlStr, "assignee = currentUser()") {
		t.Errorf("expected 'assignee = currentUser()' in JQL, got: %v", allArgs[1])
	}
}

func TestListTicketsThreeMonthFloorOverridesOldSince(t *testing.T) {
	var allArgs [][]string
	runner := sequentialRunnerWithArgs(&allArgs, []runnerResponse{
		{stdout: []byte(boardGetForProject), exitCode: 0},
		{stdout: []byte(`[]`), exitCode: 0},
	})
	a := &JiraAdapter{cfg: Config{Site: "test.atlassian.net"}, runner: runner}

	// since is older than 3 months — floor should win.
	since := time.Now().Add(-180 * 24 * time.Hour)
	_, err := a.ListTickets(context.Background(), "1", &since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	jqlStr := strings.Join(allArgs[1], " ")
	oldSinceDate := since.UTC().Format("2006-01-02")
	if strings.Contains(jqlStr, oldSinceDate) {
		t.Errorf("expected old since date %q NOT to appear (floor should win): %v",
			oldSinceDate, allArgs[1])
	}
	floor := time.Now().Add(-listTicketsWindow).UTC().Format("2006-01-02")
	if !strings.Contains(jqlStr, floor) {
		t.Errorf("expected floor date %q in JQL: %v", floor, allArgs[1])
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
			"issuetype": null,
			"status": {"id": "1", "name": "To Do"},
			"assignee": null,
			"priority": null
		}
	}]`
	runner := sequentialRunner([]runnerResponse{
		{stdout: []byte(boardGetForProject), exitCode: 0},
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
	if tickets[0].AssigneeEmail != "" {
		t.Errorf("AssigneeEmail: got %q, want empty for null assignee", tickets[0].AssigneeEmail)
	}
	if tickets[0].IssueType != "" {
		t.Errorf("IssueType: got %q, want empty for null issuetype", tickets[0].IssueType)
	}
	if tickets[0].Priority != "" {
		t.Errorf("Priority: got %q, want empty for null priority", tickets[0].Priority)
	}
}

func TestListTicketsBoardNotFound(t *testing.T) {
	// board get returns an empty JSON object when the board doesn't exist
	// (id == 0 in our acliBoardGetResult struct, which resolveProjectKey treats as not-found).
	runner := fakeRunner([]byte(`{}`), nil, 0, nil)
	a := &JiraAdapter{cfg: Config{Site: "test.atlassian.net"}, runner: runner}

	_, err := a.ListTickets(context.Background(), "999", nil)
	if err == nil {
		t.Fatal("expected error for unknown board, got nil")
	}
}

func TestResolveProjectKey_CallsBoardGetNotSearch(t *testing.T) {
	var allArgs [][]string
	runner := sequentialRunnerWithArgs(&allArgs, []runnerResponse{
		{stdout: []byte(boardGetForProject), exitCode: 0},
		{stdout: []byte(`[]`), exitCode: 0},
	})
	a := &JiraAdapter{cfg: Config{Site: "test.atlassian.net"}, runner: runner}

	if _, err := a.ListTickets(context.Background(), "1", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	first := strings.Join(allArgs[0], " ")
	if !strings.Contains(first, "board get") {
		t.Errorf("first call should be 'board get', got: %v", allArgs[0])
	}
	if strings.Contains(first, "board search") {
		t.Errorf("first call must NOT be 'board search' (hangs on accounts with many boards), got: %v", allArgs[0])
	}
	if !strings.Contains(first, "--id 1") {
		t.Errorf("first call should pass --id 1, got: %v", allArgs[0])
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
