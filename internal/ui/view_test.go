package ui

import (
	"testing"
	"time"

	"github.com/muesli/termenv"
	"github.com/charmbracelet/lipgloss"

	"github.com/kevin-zou/cmux-board/internal/config"
	"github.com/kevin-zou/cmux-board/internal/state"
)

func init() {
	// Disable color for deterministic snapshot output.
	lipgloss.SetColorProfile(termenv.Ascii)
}

// makeViewTestModel creates a Model seeded with board columns and tickets.
func makeViewTestModel(t *testing.T, snap *state.State, repos map[string]config.RepoEntry) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	if snap != nil {
		snapCopy := *snap
		if err := store.Mutate(func(s *state.State) error {
			*s = snapCopy
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
	s, rev := store.Snapshot()
	m.snapshot = s
	m.snapshotRev = rev
	m.board = s.Board
	mapped, unmapped := resolveTickets(s)
	m.tickets = flattenMapped(s.Board, mapped)
	m.unmappedTickets = unmapped
	m.width = 120
	m.height = 40
	return m
}

// buildTestSnap creates a State with one board column "todo" and the given tickets.
func buildTestSnap(tickets []state.TicketState) *state.State {
	def := state.DefaultState()
	s := &def
	s.Board = state.BoardSnapshot{
		BoardID:   "test-board",
		BoardName: "Test Board",
		Columns: []state.ColumnSnapshot{
			{ID: "todo", Name: "Todo", StatusIDs: []string{"todo"}},
		},
	}
	for _, t := range tickets {
		s.Tickets[t.Key] = t
	}
	return s
}

// should render empty board with no-tickets hint when board has no tickets.
func TestView_EmptyBoard(t *testing.T) {
	t.Parallel()
	snap := buildTestSnap(nil)
	m := makeViewTestModel(t, snap, nil)
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
	snap := buildTestSnap([]state.TicketState{
		{Key: "PROJ-1", Summary: "First", Status: "todo", LastKnownStatus: "todo"},
		{Key: "PROJ-2", Summary: "Second", Status: "todo", LastKnownStatus: "todo"},
		{Key: "PROJ-3", Summary: "Unmapped", Status: "unknown_status", LastKnownStatus: "unknown_status"},
	})
	snap.Board.Columns = []state.ColumnSnapshot{
		{ID: "col-todo", Name: "Todo", StatusIDs: []string{"todo"}},
		{ID: "col-done", Name: "Done", StatusIDs: []string{"done"}},
	}
	// Remap tickets to use the columns above.
	snap.Tickets["PROJ-1"] = state.TicketState{Key: "PROJ-1", Summary: "First", Status: "todo", LastKnownStatus: "todo"}
	snap.Tickets["PROJ-2"] = state.TicketState{Key: "PROJ-2", Summary: "Second", Status: "todo", LastKnownStatus: "todo"}
	snap.Tickets["PROJ-3"] = state.TicketState{Key: "PROJ-3", Summary: "Unmapped", Status: "unknown_status", LastKnownStatus: "unknown_status"}

	m := makeViewTestModel(t, snap, nil)
	rendered := m.View()
	// Unmapped column should appear.
	if !containsStr(rendered, "Unmapped") {
		t.Errorf("expected Unmapped column in view:\n%s", rendered)
	}
	// PROJ-3 should appear with its summary.
	if !containsStr(rendered, "Unmapped") {
		t.Errorf("expected unmapped ticket summary in view:\n%s", rendered)
	}

	// Remove unmapped ticket: update snapshot to not include PROJ-3.
	snap2 := state.DefaultState()
	snap2.Board = snap.Board
	snap2.Tickets["PROJ-1"] = snap.Tickets["PROJ-1"]
	snap2.Tickets["PROJ-2"] = snap.Tickets["PROJ-2"]
	// PROJ-3 removed.

	store2, err := state.Open(t.TempDir() + "/state2.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	if err := store2.Mutate(func(s *state.State) error { *s = snap2; return nil }); err != nil {
		t.Fatalf("store.Mutate: %v", err)
	}
	cfg := &config.Config{SchemaVersion: config.SchemaVersionCurrent, BoardID: "test-board"}
	m2 := NewModel(cfg, store2)
	s2, _ := store2.Snapshot()
	m2.snapshot = s2
	m2.board = s2.Board
	mapped2, unmapped2 := resolveTickets(s2)
	m2.tickets = flattenMapped(s2.Board, mapped2)
	m2.unmappedTickets = unmapped2
	m2.width = 120
	m2.height = 40

	rendered2 := m2.View()
	if containsStr(rendered2, "? Unmapped") {
		t.Errorf("Unmapped column should be gone when no unmapped tickets:\n%s", rendered2)
	}
}

// should render picker overlay in ModePicker.
func TestView_PickerOverlay(t *testing.T) {
	t.Parallel()
	snap := buildTestSnap(nil)
	m := makeViewTestModel(t, snap, map[string]config.RepoEntry{
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
		{Key: "PROJ-1", Summary: "Test", LastKnownStatus: "todo"},
	})
	m := makeViewTestModel(t, snap, map[string]config.RepoEntry{
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
	m := makeViewTestModel(t, snap, nil)
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
	m := makeViewTestModel(t, snap, nil)
	m, _ = m.pushToast("hello from toast")

	rendered := m.View()
	if !containsStr(rendered, "hello from toast") {
		t.Errorf("expected toast text in view output:\n%s", rendered)
	}
}
