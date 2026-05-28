package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

// makeTestModelForKeymap constructs a Model for keymap tests.
func makeTestModelForKeymap(t *testing.T) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	cfg := &config.Config{SchemaVersion: config.SchemaVersionCurrent}
	return NewModel(cfg, store)
}

// sendKey is a helper that sends a key string through handleNormalMode.
func sendKey(m Model, key string) Model {
	next, _ := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return next
}

// TestHandleNormalMode_NewApproach tests that 'N' enters ModeApproachName with input focused.
func TestHandleNormalMode_NewApproach(t *testing.T) {
	m := makeTestModelForKeymap(t)
	// Seed a ticket so the "no tickets" guard doesn't short-circuit.
	m.store.Mutate(func(s *state.State) error { //nolint:errcheck
		s.Tickets["T-1"] = state.TicketState{Key: "T-1", Status: "todo"}
		return nil
	})
	snap, rev := m.store.Snapshot()
	m.snapshot = snap
	m.snapshotRev = rev

	next, _ := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyNewApproach)})
	if next.mode != ModeApproachName {
		t.Errorf("mode = %v, want ModeApproachName", next.mode)
	}
	if !next.approachNameInput.Focused() {
		t.Error("approachNameInput should be focused after 'N'")
	}
}

// TestHandleNormalMode_Help tests that '?' enters ModeHelp.
func TestHandleNormalMode_Help(t *testing.T) {
	m := makeTestModelForKeymap(t)

	m = sendKey(m, KeyHelp)
	if m.mode != ModeHelp {
		t.Errorf("mode = %v, want ModeHelp", m.mode)
	}
}

// TestHandleNormalMode_Quit tests that 'q' sets ModeShuttingDown.
func TestHandleNormalMode_Quit(t *testing.T) {
	m := makeTestModelForKeymap(t)

	next, cmd := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyQuit)})
	if next.mode != ModeShuttingDown {
		t.Errorf("mode = %v, want ModeShuttingDown", next.mode)
	}
	if cmd == nil {
		t.Error("cmd should be non-nil (tea.Quit) after 'q'")
	}
}

// TestHandleNormalMode_ManageNoTickets tests that 'm' with no tickets is a no-op.
func TestHandleNormalMode_ManageNoTickets(t *testing.T) {
	t.Parallel()
	m := makeTestModelForKeymap(t)
	// No tickets in snapshot.

	next, _ := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyManage)})
	if next.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal (no-op with no tickets)", next.mode)
	}
}

// TestHandlePickerMode_EscClearsState tests that Esc in picker mode sets ModeNormal and nils pickerState.
func TestHandlePickerMode_EscClearsState(t *testing.T) {
	m := makeTestModelForKeymap(t)
	m.mode = ModePicker
	m.pickerState = &pickerState{cursorIdx: 2}

	next, _ := m.handlePickerMode(tea.KeyMsg{Type: tea.KeyEsc})
	if next.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", next.mode)
	}
	if next.pickerState != nil {
		t.Error("pickerState should be nil after Esc")
	}
}

// TestHandleNormalMode_ActivateNoSelection tests that Enter with no selectedKey is a no-op.
func TestHandleNormalMode_ActivateNoSelection(t *testing.T) {
	t.Parallel()
	m := makeTestModelForKeymap(t)
	// Seed a ticket so the "no tickets" guard doesn't short-circuit.
	m.store.Mutate(func(s *state.State) error { //nolint:errcheck
		s.Tickets["T-1"] = state.TicketState{Key: "T-1", Status: "todo"}
		return nil
	})
	snap, rev := m.store.Snapshot()
	m.snapshot = snap
	m.snapshotRev = rev
	m.selectedKey = ""

	next, cmd := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyEnter})
	if next.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal (no-op)", next.mode)
	}
	if cmd != nil {
		t.Error("expected nil cmd when no ticket selected")
	}
}
