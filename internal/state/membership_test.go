package state

import (
	"strings"
	"testing"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// TestImportJiraTicket_Idempotent verifies E4: calling ImportJiraTicket twice with the same
// key produces no duplicate; the second call updates tracker-owned fields but preserves X, Y,
// Source: "jira".
func TestImportJiraTicket_Idempotent(t *testing.T) {
	t.Parallel()
	s := DefaultState()

	first := tracker.Ticket{
		Key:     "PROJ-1",
		Summary: "first",
		Status:  "To Do",
		URL:     "http://first",
		Labels:  []string{"backend"},
	}
	ImportJiraTicket(&s, first)

	// Simulate user setting position.
	ts := s.Tickets["PROJ-1"]
	ts.X = 3
	ts.Y = 7
	s.Tickets["PROJ-1"] = ts

	second := tracker.Ticket{
		Key:     "PROJ-1",
		Summary: "updated",
		Status:  "In Progress",
		URL:     "http://updated",
		Labels:  []string{"frontend"},
	}
	ImportJiraTicket(&s, second)

	got := s.Tickets["PROJ-1"]
	// Tracker-owned fields updated.
	if got.Summary != "updated" {
		t.Errorf("Summary: got %q, want 'updated'", got.Summary)
	}
	if got.Status != "In Progress" {
		t.Errorf("Status: got %q, want 'In Progress'", got.Status)
	}
	if got.URL != "http://updated" {
		t.Errorf("URL: got %q, want 'http://updated'", got.URL)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "frontend" {
		t.Errorf("Labels: got %v, want ['frontend']", got.Labels)
	}
	// Local-only fields preserved.
	if got.X != 3 || got.Y != 7 {
		t.Errorf("(X,Y): got (%d,%d), want (3,7)", got.X, got.Y)
	}
	if got.Source != "jira" {
		t.Errorf("Source: got %q, want 'jira'", got.Source)
	}
	// Only one ticket in state.
	if len(s.Tickets) != 1 {
		t.Errorf("len(Tickets): got %d, want 1 (no duplicate)", len(s.Tickets))
	}
}

// TestImportJiraTicket_PopulatesURLAndLabels verifies Review F-02 cross-check: URL and Labels
// from tracker.Ticket flow into TicketState.
func TestImportJiraTicket_PopulatesURLAndLabels(t *testing.T) {
	t.Parallel()
	s := DefaultState()
	tk := tracker.Ticket{
		Key:    "PROJ-2",
		URL:    "https://jira.example.com/browse/PROJ-2",
		Labels: []string{"p0", "security"},
	}
	ImportJiraTicket(&s, tk)

	got := s.Tickets["PROJ-2"]
	if got.URL != "https://jira.example.com/browse/PROJ-2" {
		t.Errorf("URL: got %q, want 'https://jira.example.com/browse/PROJ-2'", got.URL)
	}
	if len(got.Labels) != 2 || got.Labels[0] != "p0" || got.Labels[1] != "security" {
		t.Errorf("Labels: got %v, want ['p0','security']", got.Labels)
	}
}

// TestCreateLocalTicket_UniqueKeysOnDuplicateNames verifies Review F-08: two calls with the
// same name produce two distinct LOCAL-* keys; both persist and neither is overwritten.
func TestCreateLocalTicket_UniqueKeysOnDuplicateNames(t *testing.T) {
	t.Parallel()
	s := DefaultState()

	key1 := CreateLocalTicket(&s, "x")
	key2 := CreateLocalTicket(&s, "x")

	if key1 == key2 {
		t.Errorf("duplicate keys produced for same name: both = %q", key1)
	}
	if !strings.HasPrefix(key1, "LOCAL-") {
		t.Errorf("key1 %q does not start with LOCAL-", key1)
	}
	if !strings.HasPrefix(key2, "LOCAL-") {
		t.Errorf("key2 %q does not start with LOCAL-", key2)
	}
	if len(s.Tickets) != 2 {
		t.Errorf("len(Tickets): got %d, want 2", len(s.Tickets))
	}
	if _, ok := s.Tickets[key1]; !ok {
		t.Errorf("ticket %q not found in state", key1)
	}
	if _, ok := s.Tickets[key2]; !ok {
		t.Errorf("ticket %q not found in state", key2)
	}
}

// TestCycleStatus_LeavesPositionUnchanged verifies F6: cycling status does not modify X or Y.
func TestCycleStatus_LeavesPositionUnchanged(t *testing.T) {
	t.Parallel()
	s := DefaultState()
	s.Tickets["LOCAL-1"] = TicketState{
		Key:         "LOCAL-1",
		Source:      "local",
		LocalStatus: "Open",
		X:           4,
		Y:           9,
	}

	CycleStatus(&s, "LOCAL-1")

	got := s.Tickets["LOCAL-1"]
	if got.LocalStatus != "In Progress" {
		t.Errorf("LocalStatus after first cycle: got %q, want 'In Progress'", got.LocalStatus)
	}
	if got.X != 4 || got.Y != 9 {
		t.Errorf("(X,Y) changed: got (%d,%d), want (4,9)", got.X, got.Y)
	}

	CycleStatus(&s, "LOCAL-1")
	got = s.Tickets["LOCAL-1"]
	if got.LocalStatus != "Done" {
		t.Errorf("LocalStatus after second cycle: got %q, want 'Done'", got.LocalStatus)
	}
	if got.X != 4 || got.Y != 9 {
		t.Errorf("(X,Y) changed after second cycle: got (%d,%d), want (4,9)", got.X, got.Y)
	}

	CycleStatus(&s, "LOCAL-1")
	got = s.Tickets["LOCAL-1"]
	if got.LocalStatus != "Open" {
		t.Errorf("LocalStatus after third cycle (wrap): got %q, want 'Open'", got.LocalStatus)
	}
}

// TestSetTicketPosition_PersistsXY verifies F4/F5: SetTicketPosition writes new X, Y to
// the existing entry without altering other fields.
func TestSetTicketPosition_PersistsXY(t *testing.T) {
	t.Parallel()
	s := DefaultState()
	s.Tickets["PROJ-3"] = TicketState{
		Key:     "PROJ-3",
		Source:  "jira",
		Summary: "keep me",
		X:       0,
		Y:       0,
	}

	SetTicketPosition(&s, "PROJ-3", 5, 10)

	got := s.Tickets["PROJ-3"]
	if got.X != 5 || got.Y != 10 {
		t.Errorf("(X,Y): got (%d,%d), want (5,10)", got.X, got.Y)
	}
	if got.Summary != "keep me" {
		t.Errorf("Summary changed unexpectedly: got %q", got.Summary)
	}
}

// TestRemoveTicket_DeletesByKey verifies that RemoveTicket removes a key and is a no-op for
// absent keys.
func TestRemoveTicket_DeletesByKey(t *testing.T) {
	t.Parallel()
	s := DefaultState()
	s.Tickets["PROJ-4"] = TicketState{Key: "PROJ-4", Source: "jira"}

	RemoveTicket(&s, "PROJ-4")
	if _, ok := s.Tickets["PROJ-4"]; ok {
		t.Error("PROJ-4 should have been deleted")
	}

	// No-op for absent key.
	RemoveTicket(&s, "does-not-exist")
}

// TestCycleStatus_JiraTicketUsesStatus verifies that jira tickets cycle Status (not LocalStatus).
func TestCycleStatus_JiraTicketUsesStatus(t *testing.T) {
	t.Parallel()
	s := DefaultState()
	s.Tickets["PROJ-5"] = TicketState{
		Key:    "PROJ-5",
		Source: "jira",
		Status: "Open",
	}

	CycleStatus(&s, "PROJ-5")

	got := s.Tickets["PROJ-5"]
	if got.Status != "In Progress" {
		t.Errorf("Status: got %q, want 'In Progress'", got.Status)
	}
	if got.LocalStatus != "" {
		t.Errorf("LocalStatus should remain empty for jira ticket, got %q", got.LocalStatus)
	}
}

// TestSetTicketPosition_NoopForAbsentKey verifies SetTicketPosition does not create a ticket.
func TestSetTicketPosition_NoopForAbsentKey(t *testing.T) {
	t.Parallel()
	s := DefaultState()

	SetTicketPosition(&s, "ABSENT", 1, 2)

	if len(s.Tickets) != 0 {
		t.Errorf("SetTicketPosition should not create tickets; got %d", len(s.Tickets))
	}
}

// TestCycleStatus_NoopForAbsentKey verifies CycleStatus does not create a ticket.
func TestCycleStatus_NoopForAbsentKey(t *testing.T) {
	t.Parallel()
	s := DefaultState()

	CycleStatus(&s, "ABSENT")

	if len(s.Tickets) != 0 {
		t.Errorf("CycleStatus should not create tickets; got %d", len(s.Tickets))
	}
}
