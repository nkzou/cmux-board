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

	// Write initial state (tickets only; board layout is not stored on State in schema v3).
	err = store.Mutate(func(s *state.State) error {
		for k, v := range tickets {
			s.Tickets[k] = v
		}
		return nil
	})
	if err != nil {
		t.Fatalf("store.Mutate: %v", err)
	}

	cfg := &config.Config{SchemaVersion: config.SchemaVersionCurrent}
	m := NewModel(cfg, store)
	// Set board layout directly on the model (schema v3: board not stored on State).
	m.board = state.BoardSnapshot{Columns: columns}
	// Refresh so ticket slices are populated.
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

// TestHandleNormalMode_ManageNoRepo tests that 'm' with no assigned repos pushes a toast.
func TestHandleNormalMode_ManageNoRepo(t *testing.T) {
	t.Parallel()
	m := makeTestModelWithBoard(t, 1, 1)
	// No repos in cfg, no assignments.

	next, _ := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyManage)})
	if next.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", next.mode)
	}
	if len(next.toasts) == 0 {
		t.Error("expected toast when no repos assigned")
	}
}

// TestHandleNormalMode_ManageOneRepo tests that 'm' with one assigned repo opens ModePicker.
func TestHandleNormalMode_ManageOneRepo(t *testing.T) {
	t.Parallel()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	if err := store.Mutate(func(s *state.State) error {
		s.Tickets["T-1"] = state.TicketState{Key: "T-1", Status: "todo", AssignedRepoIDs: []string{"repo-a"}}
		return nil
	}); err != nil {
		t.Fatalf("store.Mutate: %v", err)
	}
	cfg := &config.Config{
		SchemaVersion: config.SchemaVersionCurrent,
		Repos:         map[string]config.RepoEntry{"repo-a": {ID: "repo-a", Name: "Repo A"}},
	}
	m := NewModel(cfg, store)
	m.board = state.BoardSnapshot{Columns: []state.ColumnSnapshot{
		{ID: "col", Name: "Col", StatusIDs: []string{"todo"}},
	}}
	m, _ = m.refreshSnapshot()

	next, _ := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyManage)})
	if next.mode != ModePicker {
		t.Errorf("mode = %v, want ModePicker", next.mode)
	}
	if next.pickerState == nil {
		t.Fatal("pickerState is nil after 'm'")
	}
	if next.pickerState.ticketID != "T-1" {
		t.Errorf("pickerState.ticketID = %q, want T-1", next.pickerState.ticketID)
	}
	if next.pickerState.repoID != "repo-a" {
		t.Errorf("pickerState.repoID = %q, want repo-a", next.pickerState.repoID)
	}
}

// TestHandleNormalMode_ManageMultiRepo tests that 'm' with multiple assigned repos opens ModeRepoPicker
// with manageIntent set.
func TestHandleNormalMode_ManageMultiRepo(t *testing.T) {
	t.Parallel()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	if err := store.Mutate(func(s *state.State) error {
		s.Tickets["T-1"] = state.TicketState{Key: "T-1", Status: "todo", AssignedRepoIDs: []string{"repo-a", "repo-b"}}
		return nil
	}); err != nil {
		t.Fatalf("store.Mutate: %v", err)
	}
	cfg := &config.Config{
		SchemaVersion: config.SchemaVersionCurrent,
		Repos: map[string]config.RepoEntry{
			"repo-a": {ID: "repo-a", Name: "Repo A"},
			"repo-b": {ID: "repo-b", Name: "Repo B"},
		},
	}
	m := NewModel(cfg, store)
	m.board = state.BoardSnapshot{Columns: []state.ColumnSnapshot{
		{ID: "col", Name: "Col", StatusIDs: []string{"todo"}},
	}}
	m, _ = m.refreshSnapshot()

	next, _ := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyManage)})
	if next.mode != ModeRepoPicker {
		t.Errorf("mode = %v, want ModeRepoPicker", next.mode)
	}
	if next.repoPicker == nil {
		t.Fatal("repoPicker is nil after 'm' with multi-repo")
	}
	if !next.repoPicker.manageIntent {
		t.Error("repoPicker.manageIntent should be true")
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
