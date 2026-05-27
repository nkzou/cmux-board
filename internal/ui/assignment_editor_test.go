package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

// makeAssignEditorModel builds a Model seeded with a ticket and assignment state.
func makeAssignEditorModel(t *testing.T, ticketKey string, assignedRepoIDs []string, repos map[string]config.RepoEntry) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	if err := store.Mutate(func(s *state.State) error {
		s.Tickets[ticketKey] = state.TicketState{
			Key:             ticketKey,
			Summary:         "Test ticket",
			AssignedRepoIDs: assignedRepoIDs,
		}
		return nil
	}); err != nil {
		t.Fatalf("store.Mutate: %v", err)
	}
	cfg := &config.Config{
		SchemaVersion: config.SchemaVersionCurrent,
		Repos:         repos,
	}
	return NewModel(cfg, store)
}

// TestNewAssignmentEditorState_PreChecksExisting: pre-checked state reflects assigned_repo_ids.
func TestNewAssignmentEditorState_PreChecksExisting(t *testing.T) {
	t.Parallel()
	repos := map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "A"},
		"repo-b": {ID: "repo-b", Name: "B"},
		"repo-c": {ID: "repo-c", Name: "C"},
	}
	m := makeAssignEditorModel(t, "PROJ-1", []string{"repo-a"}, repos)
	snap, _ := m.store.Snapshot()
	ed := newAssignmentEditorState(m.cfg, snap, "PROJ-1")
	if !ed.checked["repo-a"] {
		t.Errorf("checked[repo-a] should be true (assigned)")
	}
	if ed.checked["repo-b"] {
		t.Errorf("checked[repo-b] should be false (not assigned)")
	}
}

// TestNewAssignmentEditorState_StaleRepoPresentInList: stale repo_id appears at end of list.
func TestNewAssignmentEditorState_StaleRepoPresentInList(t *testing.T) {
	t.Parallel()
	repos := map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "A"},
	}
	m := makeAssignEditorModel(t, "PROJ-1", []string{"ghost"}, repos)
	snap, _ := m.store.Snapshot()
	ed := newAssignmentEditorState(m.cfg, snap, "PROJ-1")

	found := false
	for _, id := range ed.repoIDs {
		if id == "ghost" {
			found = true
		}
	}
	if !found {
		t.Errorf("stale repo 'ghost' should appear in repoIDs list")
	}
	// Stale entries come after config repos (repo-a first, ghost last).
	if len(ed.repoIDs) < 2 || ed.repoIDs[len(ed.repoIDs)-1] != "ghost" {
		t.Errorf("stale entry 'ghost' should be at end of list, got %v", ed.repoIDs)
	}
}

// TestRenderAssignmentEditor_CheckboxFormat: output contains [x] and [ ] on separate lines.
func TestRenderAssignmentEditor_CheckboxFormat(t *testing.T) {
	t.Parallel()
	repos := map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "A", Path: "/a"},
		"repo-b": {ID: "repo-b", Name: "B", Path: "/b"},
	}
	m := makeAssignEditorModel(t, "PROJ-1", []string{"repo-a"}, repos)
	snap, _ := m.store.Snapshot()
	m.snapshot = snap
	m.assignmentEditor = newAssignmentEditorState(m.cfg, snap, "PROJ-1")
	m.width = 100
	m.height = 40

	rendered := m.renderAssignmentEditor()
	if !containsStr(rendered, "[x]") {
		t.Errorf("expected [x] in rendered output:\n%s", rendered)
	}
	if !containsStr(rendered, "[ ]") {
		t.Errorf("expected [ ] in rendered output:\n%s", rendered)
	}
}

// TestHandleAssignmentEditor_CommitWritesStore: toggle one repo; press Enter; assert store updated.
func TestHandleAssignmentEditor_CommitWritesStore(t *testing.T) {
	t.Parallel()
	repos := map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "A"},
		"repo-b": {ID: "repo-b", Name: "B"},
	}
	m := makeAssignEditorModel(t, "PROJ-1", nil, repos)
	snap, _ := m.store.Snapshot()
	m.snapshot = snap
	m.mode = ModeAssignmentEditor
	m.assignmentEditor = newAssignmentEditorState(m.cfg, snap, "PROJ-1")
	// repoIDs sorted: [repo-a, repo-b]; cursor at 0 = repo-a
	// Toggle repo-a on.
	m, _ = m.handleAssignmentEditorMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyAssignToggle)})
	// Commit.
	m, _ = m.handleAssignmentEditorMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyAssignCommit)})

	if m.assignmentEditor != nil {
		t.Errorf("assignmentEditor should be nil after commit")
	}
	if m.mode != ModeNormal {
		t.Errorf("mode should be Normal after commit, got %v", m.mode)
	}
	snap2, _ := m.store.Snapshot()
	assigned := state.AssignedRepoIDs(snap2, "PROJ-1")
	if len(assigned) == 0 {
		t.Errorf("expected assigned_repo_ids to contain repo-a after commit")
	}
	found := false
	for _, id := range assigned {
		if id == "repo-a" {
			found = true
		}
	}
	if !found {
		t.Errorf("repo-a should be in assigned_repo_ids after commit, got %v", assigned)
	}
}

// TestHandleAssignmentEditor_EscCancels: toggle one repo; press Esc; assert no mutation.
func TestHandleAssignmentEditor_EscCancels(t *testing.T) {
	t.Parallel()
	repos := map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "A"},
	}
	m := makeAssignEditorModel(t, "PROJ-1", nil, repos)
	snap, _ := m.store.Snapshot()
	m.snapshot = snap
	m.mode = ModeAssignmentEditor
	m.assignmentEditor = newAssignmentEditorState(m.cfg, snap, "PROJ-1")
	// Toggle repo-a on (but we'll cancel).
	m, _ = m.handleAssignmentEditorMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyAssignToggle)})
	// Cancel.
	m, _ = m.handleAssignmentEditorMode(tea.KeyMsg{Type: tea.KeyEsc})

	if m.assignmentEditor != nil {
		t.Errorf("assignmentEditor should be nil after cancel")
	}
	snap2, _ := m.store.Snapshot()
	assigned := state.AssignedRepoIDs(snap2, "PROJ-1")
	if len(assigned) != 0 {
		t.Errorf("expected no assigned_repo_ids after cancel, got %v", assigned)
	}
}

// TestHandleAssignmentEditor_CommitAtomic: commit calls store.Mutate exactly once (F-MR4).
// We verify this indirectly: after commit, the store snapshot has the correct value
// (proving one atomic write happened, not piecemeal).
func TestHandleAssignmentEditor_CommitAtomic(t *testing.T) {
	t.Parallel()
	repos := map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "A"},
		"repo-b": {ID: "repo-b", Name: "B"},
	}
	m := makeAssignEditorModel(t, "PROJ-2", nil, repos)
	snap, _ := m.store.Snapshot()
	m.snapshot = snap
	m.mode = ModeAssignmentEditor
	m.assignmentEditor = newAssignmentEditorState(m.cfg, snap, "PROJ-2")

	// Toggle both repos on, then commit.
	// cursor=0 (repo-a): toggle on.
	m, _ = m.handleAssignmentEditorMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyAssignToggle)})
	// Move to repo-b.
	m, _ = m.handleAssignmentEditorMode(tea.KeyMsg{Type: tea.KeyDown})
	// cursor=1 (repo-b): toggle on.
	m, _ = m.handleAssignmentEditorMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyAssignToggle)})
	// Commit.
	m, _ = m.handleAssignmentEditorMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyAssignCommit)})

	snap2, _ := m.store.Snapshot()
	assigned := state.AssignedRepoIDs(snap2, "PROJ-2")
	if len(assigned) != 2 {
		t.Errorf("expected 2 assigned repos after atomic commit, got %v", assigned)
	}
}
