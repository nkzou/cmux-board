package ui

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// mockTracker is a minimal IssueTracker for import overlay tests.
type mockTracker struct {
	getTicketFn func(ctx context.Context, key string) (tracker.Ticket, error)
}

func (mt *mockTracker) WhoAmI(_ context.Context) (tracker.UserIdentity, error) {
	return tracker.UserIdentity{}, nil
}
func (mt *mockTracker) ListBoards(_ context.Context) ([]tracker.BoardSummary, error) {
	return nil, nil
}
func (mt *mockTracker) GetBoard(_ context.Context, _ string) (tracker.Board, error) {
	return tracker.Board{}, nil
}
func (mt *mockTracker) ListTickets(_ context.Context, _ string, _ *time.Time) ([]tracker.Ticket, error) {
	return nil, nil
}
func (mt *mockTracker) GetTicket(ctx context.Context, key string) (tracker.Ticket, error) {
	if mt.getTicketFn != nil {
		return mt.getTicketFn(ctx, key)
	}
	return tracker.Ticket{}, errors.New("not configured")
}
func (mt *mockTracker) TransitionStatus(_ context.Context, _, _, _ string) error {
	return nil
}
func (mt *mockTracker) Capabilities() tracker.Capabilities {
	return tracker.Capabilities{}
}

// makeImportModel builds a model with a mock tracker and the given initial mode.
func makeImportModel(t *testing.T, tr tracker.IssueTracker) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	m := NewModelWithTracker(context.Background(), nil, store, tr)
	return m
}

// drainCmd executes a tea.Cmd and returns the resulting tea.Msg, or nil if cmd is nil.
func drainCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// TestImport_CommitCreatesJiraTicket verifies that confirming an import key creates
// a ticket with source "jira", correct URL, and Labels populated.
func TestImport_CommitCreatesJiraTicket(t *testing.T) {
	t.Parallel()
	fixedTicket := tracker.Ticket{
		Key:     "PROJ-1",
		Summary: "Fix login bug",
		Status:  "In Progress",
		URL:     "https://example.atlassian.net/browse/PROJ-1",
		Labels:  []string{"frontend", "auth"},
	}
	tr := &mockTracker{
		getTicketFn: func(_ context.Context, key string) (tracker.Ticket, error) {
			if key == "PROJ-1" {
				return fixedTicket, nil
			}
			return tracker.Ticket{}, errors.New("not found")
		},
	}
	m := makeImportModel(t, tr)
	m.mode = ModeImportInput
	m.importInput.SetValue("PROJ-1")

	_, cmd := m.handleImportInputMode(tea.KeyMsg{Type: tea.KeyEnter})
	msg := drainCmd(cmd)
	// Feed the result message back into the model.
	if msg != nil {
		raw, _ := m.Update(msg)
		m = raw.(Model) //nolint:forcetypeassert
	}

	snap, _ := m.store.Snapshot()
	ts, ok := snap.Tickets["PROJ-1"]
	if !ok {
		t.Fatal("ticket PROJ-1 not found in state")
	}
	if ts.Source != "jira" {
		t.Errorf("Source = %q, want %q", ts.Source, "jira")
	}
	if ts.URL != fixedTicket.URL {
		t.Errorf("URL = %q, want %q", ts.URL, fixedTicket.URL)
	}
	if len(ts.Labels) != 2 {
		t.Errorf("Labels = %v, want 2 elements", ts.Labels)
	}
}

// TestImport_DuplicateKeyIsNoOp verifies E4: importing the same key twice produces
// one ticket and no error.
func TestImport_DuplicateKeyIsNoOp(t *testing.T) {
	t.Parallel()
	fixedTicket := tracker.Ticket{
		Key:     "PROJ-2",
		Summary: "Duplicate check",
		Status:  "Open",
		URL:     "https://example.atlassian.net/browse/PROJ-2",
	}
	tr := &mockTracker{
		getTicketFn: func(_ context.Context, _ string) (tracker.Ticket, error) {
			return fixedTicket, nil
		},
	}
	m := makeImportModel(t, tr)

	// First import.
	m.mode = ModeImportInput
	m.importInput.SetValue("PROJ-2")
	_, cmd := m.handleImportInputMode(tea.KeyMsg{Type: tea.KeyEnter})
	msg := drainCmd(cmd)
	if msg != nil {
		raw, _ := m.Update(msg)
		m = raw.(Model) //nolint:forcetypeassert
	}

	// Second import of same key.
	m.mode = ModeImportInput
	m.importInput.SetValue("PROJ-2")
	_, cmd2 := m.handleImportInputMode(tea.KeyMsg{Type: tea.KeyEnter})
	msg2 := drainCmd(cmd2)
	if msg2 != nil {
		raw, _ := m.Update(msg2)
		m = raw.(Model) //nolint:forcetypeassert
	}

	snap, _ := m.store.Snapshot()
	count := 0
	for range snap.Tickets {
		count++
	}
	if count != 1 {
		t.Errorf("ticket count = %d, want 1 (no duplicate)", count)
	}
}

// TestImport_CancelIsNoop verifies that Esc returns to ModeNormal with no state changes.
func TestImport_CancelIsNoop(t *testing.T) {
	t.Parallel()
	m := makeImportModel(t, nil)
	m.mode = ModeImportInput
	m.importInput.SetValue("PROJ-99")

	snapBefore, _ := m.store.Snapshot()
	countBefore := len(snapBefore.Tickets)

	next, cmd := m.handleImportInputMode(tea.KeyMsg{Type: tea.KeyEsc})

	if next.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal after Esc", next.mode)
	}
	if cmd != nil {
		t.Error("cmd should be nil on cancel")
	}
	snap, _ := m.store.Snapshot()
	if len(snap.Tickets) != countBefore {
		t.Errorf("ticket count changed on cancel: got %d, want %d", len(snap.Tickets), countBefore)
	}
}

// TestImport_TrackerErrorEmitsErrMsg verifies that GetTicket errors produce a PollErrMsg.
func TestImport_TrackerErrorEmitsErrMsg(t *testing.T) {
	t.Parallel()
	tr := &mockTracker{
		getTicketFn: func(_ context.Context, _ string) (tracker.Ticket, error) {
			return tracker.Ticket{}, errors.New("network error")
		},
	}
	m := makeImportModel(t, tr)
	m.mode = ModeImportInput
	m.importInput.SetValue("PROJ-BAD")

	next, cmd := m.handleImportInputMode(tea.KeyMsg{Type: tea.KeyEnter})

	if next.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal after Enter", next.mode)
	}
	msg := drainCmd(cmd)
	if msg == nil {
		t.Fatal("cmd returned nil, want PollErrMsg")
	}
	if _, ok := msg.(PollErrMsg); !ok {
		t.Errorf("msg type = %T, want PollErrMsg", msg)
	}
}
