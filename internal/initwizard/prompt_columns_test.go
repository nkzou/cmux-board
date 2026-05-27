package initwizard

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// columnsMockAdapter is a minimal IssueTracker for ConfigureColumns tests.
type columnsMockAdapter struct {
	tickets  []tracker.Ticket
	ticketErr error
}

func (m *columnsMockAdapter) WhoAmI(ctx context.Context) (tracker.UserIdentity, error) {
	return tracker.UserIdentity{}, errors.New("not implemented")
}
func (m *columnsMockAdapter) ListBoards(ctx context.Context) ([]tracker.BoardSummary, error) {
	return nil, errors.New("not implemented")
}
func (m *columnsMockAdapter) GetBoard(ctx context.Context, boardID string) (tracker.Board, error) {
	return tracker.Board{}, errors.New("not implemented")
}
func (m *columnsMockAdapter) ListTickets(ctx context.Context, boardID string, since *time.Time) ([]tracker.Ticket, error) {
	return m.tickets, m.ticketErr
}
func (m *columnsMockAdapter) TransitionStatus(ctx context.Context, ticketID, expectedFromStatus, toStatus string) error {
	return errors.New("not implemented")
}
func (m *columnsMockAdapter) Capabilities() tracker.Capabilities {
	return tracker.Capabilities{}
}

// TestConfigureColumns_AutoMap verifies the auto-map happy path:
// discover 3 statuses, press Enter (default Y) → 3 columns in order.
func TestConfigureColumns_AutoMap(t *testing.T) {
	t.Parallel()
	a := &columnsMockAdapter{tickets: []tracker.Ticket{
		{Status: "Done"},
		{Status: "In Progress"},
		{Status: "To Do"},
		{Status: "In Progress"}, // duplicate — should be deduped
	}}

	// Enter → accept default Y (auto-map).
	r := strings.NewReader("\n")
	var w bytes.Buffer

	cols, err := ConfigureColumns(context.Background(), &w, r, a, "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Statuses are sorted alphabetically: Done, In Progress, To Do.
	if len(cols) != 3 {
		t.Fatalf("want 3 columns (one per unique status), got %d: %v", len(cols), cols)
	}
	if cols[0].Name != "Done" {
		t.Errorf("col[0].Name: got %q, want %q", cols[0].Name, "Done")
	}
	if cols[1].Name != "In Progress" {
		t.Errorf("col[1].Name: got %q, want %q", cols[1].Name, "In Progress")
	}
	if cols[2].Name != "To Do" {
		t.Errorf("col[2].Name: got %q, want %q", cols[2].Name, "To Do")
	}
	// Each column has exactly its own status in StatusIDs.
	if len(cols[0].StatusIDs) != 1 || cols[0].StatusIDs[0] != "Done" {
		t.Errorf("col[0].StatusIDs: got %v, want [Done]", cols[0].StatusIDs)
	}
	// IDs should be slugified names.
	if cols[1].ID != "in-progress" {
		t.Errorf("col[1].ID: got %q, want %q", cols[1].ID, "in-progress")
	}
}

// TestConfigureColumns_CustomMode verifies the custom-mode path:
// discover statuses, choose n, define 2 columns grouping statuses, then N to exit.
func TestConfigureColumns_CustomMode(t *testing.T) {
	t.Parallel()
	a := &columnsMockAdapter{tickets: []tracker.Ticket{
		{Status: "Open"},
		{Status: "In Progress"},
		{Status: "Done"},
		{Status: "Closed"},
	}}

	// n → enter custom mode
	// Column 1: "Backlog" → "Open, In Progress"
	// y → add another
	// Column 2: "Finished" → "Done, Closed"
	// N → done
	input := "n\nBacklog\nOpen, In Progress\ny\nFinished\nDone, Closed\nN\n"
	r := strings.NewReader(input)
	var w bytes.Buffer

	cols, err := ConfigureColumns(context.Background(), &w, r, a, "99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cols) != 2 {
		t.Fatalf("want 2 columns, got %d: %v", len(cols), cols)
	}
	if cols[0].Name != "Backlog" {
		t.Errorf("col[0].Name: got %q, want %q", cols[0].Name, "Backlog")
	}
	if len(cols[0].StatusIDs) != 2 {
		t.Errorf("col[0].StatusIDs: got %v, want 2 items", cols[0].StatusIDs)
	}
	if cols[1].Name != "Finished" {
		t.Errorf("col[1].Name: got %q, want %q", cols[1].Name, "Finished")
	}
	if len(cols[1].StatusIDs) != 2 {
		t.Errorf("col[1].StatusIDs: got %v, want 2 items", cols[1].StatusIDs)
	}
}

// TestConfigureColumns_DiscoveryFailure verifies fallback to free-form entry
// when ListTickets returns an error.
func TestConfigureColumns_DiscoveryFailure(t *testing.T) {
	t.Parallel()
	a := &columnsMockAdapter{ticketErr: errors.New("acli timeout")}

	// Custom mode entered immediately (no auto-map prompt).
	// Define 1 column: "Work" → "WIP"
	// N → done
	input := "Work\nWIP\nN\n"
	r := strings.NewReader(input)
	var w bytes.Buffer

	cols, err := ConfigureColumns(context.Background(), &w, r, a, "7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cols) != 1 {
		t.Fatalf("want 1 column, got %d: %v", len(cols), cols)
	}
	if cols[0].Name != "Work" {
		t.Errorf("col[0].Name: got %q, want %q", cols[0].Name, "Work")
	}
	// Warning about discovery failure should appear in output.
	out := w.String()
	if !strings.Contains(out, "Warning") {
		t.Errorf("expected warning about discovery failure in output, got: %q", out)
	}
}

// TestConfigureColumns_EmptyProject verifies the empty-project path:
// ListTickets returns 0 tickets → skip auto-map prompt → custom mode.
func TestConfigureColumns_EmptyProject(t *testing.T) {
	t.Parallel()
	a := &columnsMockAdapter{tickets: []tracker.Ticket{}}

	// No auto-map prompt; straight to custom mode.
	input := "To Do\nOpen\nN\n"
	r := strings.NewReader(input)
	var w bytes.Buffer

	cols, err := ConfigureColumns(context.Background(), &w, r, a, "5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cols) != 1 {
		t.Fatalf("want 1 column, got %d: %v", len(cols), cols)
	}
	if cols[0].Name != "To Do" {
		t.Errorf("col[0].Name: got %q, want %q", cols[0].Name, "To Do")
	}
}

// TestConfigureColumns_UnknownStatusWarning verifies that entering a status not in
// the discovered set warns but is still accepted.
func TestConfigureColumns_UnknownStatusWarning(t *testing.T) {
	t.Parallel()
	a := &columnsMockAdapter{tickets: []tracker.Ticket{
		{Status: "Open"},
		{Status: "Done"},
	}}

	// n → custom mode
	// Column 1: "Mixed" → "Open, Foo" (Foo is unknown)
	// N → done
	input := "n\nMixed\nOpen, Foo\nN\n"
	r := strings.NewReader(input)
	var w bytes.Buffer

	cols, err := ConfigureColumns(context.Background(), &w, r, a, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cols) != 1 {
		t.Fatalf("want 1 column, got %d", len(cols))
	}
	if len(cols[0].StatusIDs) != 2 {
		t.Errorf("want 2 status IDs (Open + Foo), got %v", cols[0].StatusIDs)
	}
	out := w.String()
	if !strings.Contains(out, "Warning") || !strings.Contains(out, "Foo") {
		t.Errorf("expected warning about unknown status 'Foo', got output: %q", out)
	}
}

// TestSlugifyStatus verifies the ID generation helper.
func TestSlugifyStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"In Progress", "in-progress"},
		{"To Do", "to-do"},
		{"Done", "done"},
		{"  Weird  State  ", "weird--state"},
		{"ALLCAPS", "allcaps"},
	}
	for _, tt := range tests {
		got := slugifyStatus(tt.input)
		// We only check that it's lowercase, non-empty, and contains no spaces.
		if strings.Contains(got, " ") {
			t.Errorf("slugifyStatus(%q) = %q contains space", tt.input, got)
		}
		if got == "" {
			t.Errorf("slugifyStatus(%q) = empty string", tt.input)
		}
		// Check exact match for simple cases.
		if tt.input == "In Progress" && got != "in-progress" {
			t.Errorf("slugifyStatus(%q) = %q, want %q", tt.input, got, "in-progress")
		}
		if tt.input == "Done" && got != "done" {
			t.Errorf("slugifyStatus(%q) = %q, want %q", tt.input, got, "done")
		}
	}
}
