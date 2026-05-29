package ui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

// makeCreateModel constructs a Model for create-input overlay tests.
func makeCreateModel(t *testing.T) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	cfg := &config.Config{SchemaVersion: config.SchemaVersionCurrent}
	return NewModelWithContext(context.Background(), cfg, store)
}

// TestCreate_CommitCreatesLocalTicket verifies F2: creating a local ticket persists
// a ticket with source "local", the given name in Summary, a LOCAL-* key, and
// LocalStatus "Open" at position (0, 0).
func TestCreate_CommitCreatesLocalTicket(t *testing.T) {
	t.Parallel()
	m := makeCreateModel(t)
	m.mode = ModeCreateInput
	m.createInput.SetValue("My new task")

	next, _ := m.handleCreateInputMode(tea.KeyMsg{Type: tea.KeyEnter})

	if next.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", next.mode)
	}

	snap, _ := m.store.Snapshot()
	if len(snap.Tickets) != 1 {
		t.Fatalf("ticket count = %d, want 1", len(snap.Tickets))
	}
	var ts state.TicketState
	for _, v := range snap.Tickets {
		ts = v
	}
	if ts.Source != "local" {
		t.Errorf("Source = %q, want %q", ts.Source, "local")
	}
	if ts.Summary != "My new task" {
		t.Errorf("Summary = %q, want %q", ts.Summary, "My new task")
	}
	if len(ts.Key) == 0 || ts.Key[:6] != "LOCAL-" {
		t.Errorf("Key = %q, want LOCAL-* prefix", ts.Key)
	}
	if ts.LocalStatus != "Open" {
		t.Errorf("LocalStatus = %q, want %q", ts.LocalStatus, "Open")
	}
	if ts.X < 0 || ts.Y < 0 {
		t.Errorf("position = (%d, %d), want non-negative cascade default", ts.X, ts.Y)
	}
}

// TestCreate_DuplicateNamesUniqueKeys verifies Review F-08: two tickets with the same
// name get distinct LOCAL-* keys.
func TestCreate_DuplicateNamesUniqueKeys(t *testing.T) {
	t.Parallel()
	m := makeCreateModel(t)

	m.mode = ModeCreateInput
	m.createInput.SetValue("x")
	m.handleCreateInputMode(tea.KeyMsg{Type: tea.KeyEnter}) //nolint:errcheck

	m.mode = ModeCreateInput
	m.createInput.SetValue("x")
	m.handleCreateInputMode(tea.KeyMsg{Type: tea.KeyEnter}) //nolint:errcheck

	snap, _ := m.store.Snapshot()
	if len(snap.Tickets) != 2 {
		t.Errorf("ticket count = %d, want 2 (distinct keys for same name)", len(snap.Tickets))
	}
}

// TestCreate_CancelIsNoop verifies that Esc returns to ModeNormal with no state change.
func TestCreate_CancelIsNoop(t *testing.T) {
	t.Parallel()
	m := makeCreateModel(t)
	m.mode = ModeCreateInput
	m.createInput.SetValue("some name")

	next, cmd := m.handleCreateInputMode(tea.KeyMsg{Type: tea.KeyEsc})

	if next.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal after Esc", next.mode)
	}
	if cmd != nil {
		t.Error("cmd should be nil on cancel")
	}
	snap, _ := m.store.Snapshot()
	if len(snap.Tickets) != 0 {
		t.Errorf("ticket count = %d, want 0 after cancel", len(snap.Tickets))
	}
}

// TestCreate_EmptyNameIsNoop verifies that committing an empty/whitespace name does nothing.
func TestCreate_EmptyNameIsNoop(t *testing.T) {
	t.Parallel()
	m := makeCreateModel(t)
	m.mode = ModeCreateInput
	m.createInput.SetValue("   ")

	next, _ := m.handleCreateInputMode(tea.KeyMsg{Type: tea.KeyEnter})

	if next.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", next.mode)
	}
	snap, _ := m.store.Snapshot()
	if len(snap.Tickets) != 0 {
		t.Errorf("ticket count = %d, want 0 for whitespace name", len(snap.Tickets))
	}
}
