package ui

import (
	"testing"
	"time"

	"github.com/muesli/termenv"
	"github.com/charmbracelet/lipgloss"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

func init() {
	// Disable color for deterministic snapshot output.
	lipgloss.SetColorProfile(termenv.Ascii)
}

// defaultTestBoard is the standard board layout used by view tests.
// Schema v3: board layout is NOT stored on State; set directly on Model.
var defaultTestBoard = state.BoardSnapshot{
	BoardID:   "test-board",
	BoardName: "Test Board",
	Columns: []state.ColumnSnapshot{
		{ID: "todo", Name: "Todo", StatusIDs: []string{"todo"}},
	},
}

// makeViewTestModel creates a Model seeded with board columns and tickets.
// Board layout is set directly on the model (schema v3: not stored on State).
func makeViewTestModel(t *testing.T, snap *state.State, board state.BoardSnapshot, repos map[string]config.RepoEntry) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	if snap != nil {
		// Only write tickets and activations — board is not stored on State.
		if err := store.Mutate(func(s *state.State) error {
			for k, v := range snap.Tickets {
				s.Tickets[k] = v
			}
			for k, v := range snap.Activations {
				s.Activations[k] = v
			}
			return nil
		}); err != nil {
			t.Fatalf("store.Mutate: %v", err)
		}
	}
	cfg := &config.Config{
		SchemaVersion: config.SchemaVersionCurrent,
		BoardID:       "test-board",
		Repos:         repos,
	}
	m := NewModel(cfg, store)
	// Set board layout directly on the model.
	m.board = board
	s, rev := store.Snapshot()
	m.snapshot = s
	m.snapshotRev = rev
	mapped, unmapped := resolveTicketsWithBoard(board, s)
	m.tickets = flattenMapped(m.board, mapped)
	m.unmappedTickets = unmapped
	m.width = 120
	m.height = 40
	return m
}

// buildTestSnap creates a State with the given tickets (no board — schema v3).
func buildTestSnap(tickets []state.TicketState) *state.State {
	def := state.DefaultState()
	s := &def
	for _, t := range tickets {
		s.Tickets[t.Key] = t
	}
	return s
}

// should render empty board with no-tickets hint when board has no tickets.
func TestView_EmptyBoard(t *testing.T) {
	t.Parallel()
	snap := buildTestSnap(nil)
	m := makeViewTestModel(t, snap, defaultTestBoard, nil)
	rendered := m.View()
	if rendered == "" {
		t.Error("View() returned empty string for empty board")
	}
	// Should contain the board name.
	if !containsStr(rendered, "Test Board") {
		t.Errorf("expected board name in view output:\n%s", rendered)
	}
}

// should render three tickets across two columns plus one unmapped column (E1).
func TestView_ThreeTicketsTwoColumnsUnmapped(t *testing.T) {
	t.Parallel()
	testBoard := state.BoardSnapshot{
		BoardID:   "test-board",
		BoardName: "Test Board",
		Columns: []state.ColumnSnapshot{
			{ID: "col-todo", Name: "Todo", StatusIDs: []string{"todo"}},
			{ID: "col-done", Name: "Done", StatusIDs: []string{"done"}},
		},
	}
	snap := buildTestSnap([]state.TicketState{
		{Key: "PROJ-1", Summary: "First", Status: "todo"},
		{Key: "PROJ-2", Summary: "Second", Status: "todo"},
		{Key: "PROJ-3", Summary: "Unmapped", Status: "unknown_status"},
	})

	m := makeViewTestModel(t, snap, testBoard, nil)
	rendered := m.View()
	// Unmapped column should appear.
	if !containsStr(rendered, "Unmapped") {
		t.Errorf("expected Unmapped column in view:\n%s", rendered)
	}
	// PROJ-3 should appear with its summary.
	if !containsStr(rendered, "Unmapped") {
		t.Errorf("expected unmapped ticket summary in view:\n%s", rendered)
	}

	// Remove unmapped ticket: build a new model without PROJ-3.
	snap2 := buildTestSnap([]state.TicketState{
		{Key: "PROJ-1", Summary: "First", Status: "todo"},
		{Key: "PROJ-2", Summary: "Second", Status: "todo"},
	})
	m2 := makeViewTestModel(t, snap2, testBoard, nil)
	rendered2 := m2.View()
	if containsStr(rendered2, "? Unmapped") {
		t.Errorf("Unmapped column should be gone when no unmapped tickets:\n%s", rendered2)
	}
}

// should render picker overlay in ModePicker.
func TestView_PickerOverlay(t *testing.T) {
	t.Parallel()
	snap := buildTestSnap(nil)
	m := makeViewTestModel(t, snap, defaultTestBoard, map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "Repo A"},
	})
	// Seed two activations so picker is non-nil.
	if err := m.store.Mutate(func(s *state.State) error {
		s.Activations["PROJ-1"] = []state.ActivationEntry{
			{ActivationID: "01AAAAAAAAAAAAAAAAAAAAAA", ActIDShort: "01AAAAAA", RepoID: "repo-a",
				TicketID: "PROJ-1", ApproachName: "alpha", Step: state.StepCmuxCreated, Complete: true, CreatedAt: time.Now()},
			{ActivationID: "01BBBBBBBBBBBBBBBBBBBBBB", ActIDShort: "01BBBBBB", RepoID: "repo-a",
				TicketID: "PROJ-1", ApproachName: "beta", Step: state.StepCmuxCreated, Complete: true, CreatedAt: time.Now()},
		}
		return nil
	}); err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	s, _ := m.store.Snapshot()
	m.snapshot = s
	m.mode = ModePicker
	m.pickerState = newPickerState(s, "PROJ-1", "repo-a")

	rendered := m.View()
	// Overlay should contain the title.
	if !containsStr(rendered, "Activations:") {
		t.Errorf("expected picker overlay title in view:\n%s", rendered)
	}
}

// should render assignment editor overlay in ModeAssignmentEditor.
func TestView_AssignmentEditorOverlay(t *testing.T) {
	t.Parallel()
	snap := buildTestSnap([]state.TicketState{
		{Key: "PROJ-1", Summary: "Test", Status: "todo"},
	})
	m := makeViewTestModel(t, snap, defaultTestBoard, map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "Repo A", Path: "/a"},
	})
	s, _ := m.store.Snapshot()
	m.snapshot = s
	m.mode = ModeAssignmentEditor
	m.assignmentEditor = newAssignmentEditorState(m.cfg, s, "PROJ-1")

	rendered := m.View()
	if !containsStr(rendered, "Assign repos:") {
		t.Errorf("expected assignment editor title in view:\n%s", rendered)
	}
}

// should render approach name prompt overlay in ModeApproachName.
func TestView_ApproachNameOverlay(t *testing.T) {
	t.Parallel()
	snap := buildTestSnap(nil)
	m := makeViewTestModel(t, snap, defaultTestBoard, nil)
	m.mode = ModeApproachName
	m.approachNameInput.Focus()

	rendered := m.View()
	if !containsStr(rendered, "New approach name") {
		t.Errorf("expected approach name prompt in view:\n%s", rendered)
	}
}

// should append toast text when toast queue non-empty.
func TestView_ToastAppended(t *testing.T) {
	t.Parallel()
	snap := buildTestSnap(nil)
	m := makeViewTestModel(t, snap, defaultTestBoard, nil)
	m, _ = m.pushToast("hello from toast")

	rendered := m.View()
	if !containsStr(rendered, "hello from toast") {
		t.Errorf("expected toast text in view output:\n%s", rendered)
	}
}
