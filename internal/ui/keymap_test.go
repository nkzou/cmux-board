package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

// makeTestModelWithBoard constructs a Model with a board having the given number of
// columns, each with the given number of tickets, for navigation tests.
func makeTestModelWithBoard(t *testing.T, numCols, ticketsPerCol int) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}

	// Build a board snapshot with numCols columns.
	columns := make([]state.ColumnSnapshot, numCols)
	tickets := make(map[string]state.TicketState)
	for i := 0; i < numCols; i++ {
		colID := string(rune('A' + i))
		statusID := "status-" + colID
		columns[i] = state.ColumnSnapshot{ID: colID, Name: colID, StatusIDs: []string{statusID}}
		for j := 0; j < ticketsPerCol; j++ {
			key := colID + "-" + string(rune('0'+j))
			tickets[key] = state.TicketState{Key: key, Status: statusID}
		}
	}

	// Write initial state.
	err = store.Mutate(func(s *state.State) error {
		s.Board = state.BoardSnapshot{Columns: columns}
		s.Tickets = tickets
		return nil
	})
	if err != nil {
		t.Fatalf("store.Mutate: %v", err)
	}

	cfg := &config.Config{SchemaVersion: config.SchemaVersionCurrent}
	m := NewModel(cfg, store)
	// Refresh so board and tickets slices are populated.
	m, _ = m.refreshSnapshot()
	return m
}

// sendKey is a helper that sends a key string through handleNormalMode.
func sendKey(m Model, key string) Model {
	next, _ := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	// For special keys (enter, esc) BubbleTea uses Type != KeyRunes.
	// For simple single-char keys this path works.
	return next
}

// sendKeyMsg sends a tea.KeyMsg through handleNormalMode with the given type and alt.
func sendKeyMsg(m Model, k tea.KeyMsg) Model {
	next, _ := m.handleNormalMode(k)
	return next
}

// TestHandleNormalMode_MoveLeft tests that 'h' decrements activeColIdx.
func TestHandleNormalMode_MoveLeft(t *testing.T) {
	m := makeTestModelWithBoard(t, 3, 1)
	m.activeColIdx = 2

	m = sendKey(m, KeyLeft)
	if m.activeColIdx != 1 {
		t.Errorf("activeColIdx = %d, want 1", m.activeColIdx)
	}
}

// TestHandleNormalMode_MoveLeftClamp tests that 'h' at column 0 stays at 0.
func TestHandleNormalMode_MoveLeftClamp(t *testing.T) {
	m := makeTestModelWithBoard(t, 3, 1)
	m.activeColIdx = 0

	m = sendKey(m, KeyLeft)
	if m.activeColIdx != 0 {
		t.Errorf("activeColIdx = %d, want 0 (clamped)", m.activeColIdx)
	}
}

// TestHandleNormalMode_MoveRightClamp tests that 'l' at the last column stays there.
func TestHandleNormalMode_MoveRightClamp(t *testing.T) {
	m := makeTestModelWithBoard(t, 3, 1)
	m.activeColIdx = 2 // last column

	m = sendKey(m, KeyRight)
	if m.activeColIdx != 2 {
		t.Errorf("activeColIdx = %d, want 2 (clamped)", m.activeColIdx)
	}
}

// TestHandleNormalMode_MoveDownClamp tests that 'j' at the last ticket in a column clamps.
func TestHandleNormalMode_MoveDownClamp(t *testing.T) {
	m := makeTestModelWithBoard(t, 1, 2)
	m.activeColIdx = 0
	m.activeTicketIdx = 1 // last ticket (0-indexed in a 2-ticket column)

	m = sendKey(m, KeyDown)
	if m.activeTicketIdx != 1 {
		t.Errorf("activeTicketIdx = %d, want 1 (clamped)", m.activeTicketIdx)
	}
}

// TestHandleNormalMode_NewApproach tests that 'N' enters ModeApproachName with input focused.
func TestHandleNormalMode_NewApproach(t *testing.T) {
	m := makeTestModelWithBoard(t, 1, 1)

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
	m := makeTestModelWithBoard(t, 1, 1)

	m = sendKey(m, KeyHelp)
	if m.mode != ModeHelp {
		t.Errorf("mode = %v, want ModeHelp", m.mode)
	}
}

// TestHandleNormalMode_Quit tests that 'q' sets ModeShuttingDown.
func TestHandleNormalMode_Quit(t *testing.T) {
	m := makeTestModelWithBoard(t, 1, 1)

	next, cmd := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyQuit)})
	if next.mode != ModeShuttingDown {
		t.Errorf("mode = %v, want ModeShuttingDown", next.mode)
	}
	if cmd == nil {
		t.Error("cmd should be non-nil (tea.Quit) after 'q'")
	}
}

// TestHandlePickerMode_EscClearsState tests that Esc in picker mode sets ModeNormal and nils pickerState.
func TestHandlePickerMode_EscClearsState(t *testing.T) {
	m := makeTestModelWithBoard(t, 1, 1)
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
