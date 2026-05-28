package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

func init() {
	// Disable color for deterministic snapshot output.
	lipgloss.SetColorProfile(termenv.Ascii)
}

// makeViewTestModel creates a Model seeded with tickets.
// Board layout is removed in T-402a; these tests cover basic view composition.
func makeViewTestModel(t *testing.T, snap *state.State, repos map[string]config.RepoEntry) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	if snap != nil {
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
	s, rev := store.Snapshot()
	m.snapshot = s
	m.snapshotRev = rev
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

// should render empty view without panicking.
func TestView_EmptyBoard(t *testing.T) {
	t.Parallel()
	snap := buildTestSnap(nil)
	m := makeViewTestModel(t, snap, nil)
	rendered := m.View()
	if rendered == "" {
		t.Error("View() returned empty string for empty board")
	}
	// Should contain the board id from cfg.
	if !containsStr(rendered, "test-board") {
		t.Errorf("expected board id in view output:\n%s", rendered)
	}
}

func TestView_CanvasUsesAvailableHeight(t *testing.T) {
	t.Parallel()
	snap := buildTestSnap(nil)
	m := makeViewTestModel(t, snap, nil)
	m.height = 10

	rendered := m.View()
	lines := strings.Count(rendered, "\n") + 1
	if lines != m.height {
		t.Errorf("View() rendered %d lines, want terminal height %d", lines, m.height)
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
		{Key: "PROJ-1", Summary: "Test", Status: "todo"},
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
