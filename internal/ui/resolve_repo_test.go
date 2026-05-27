package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kevin-zou/cmux-board/internal/config"
	"github.com/kevin-zou/cmux-board/internal/state"
)

// makeResolveRepoModel builds a Model with the given ticket and assigned repos.
func makeResolveRepoModel(t *testing.T, ticketKey string, assignedRepoIDs []string, repos map[string]config.RepoEntry) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	if err := store.Mutate(func(s *state.State) error {
		s.Tickets[ticketKey] = state.TicketState{
			Key:             ticketKey,
			Summary:         "Test",
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
	m := NewModel(cfg, store)
	snap, rev := store.Snapshot()
	m.snapshot = snap
	m.snapshotRev = rev
	return m
}

// should return repoID immediately when exactly one repo assigned.
func TestResolveRepoAndRoute_OneAssigned(t *testing.T) {
	t.Parallel()
	m := makeResolveRepoModel(t, "PROJ-1", []string{"repo-a"}, map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "A"},
	})
	_, res := resolveRepoAndRoute(m, "PROJ-1")
	if res.Cancelled {
		t.Error("expected not cancelled")
	}
	if res.PickerOpened {
		t.Error("expected no picker opened for single assigned repo")
	}
	if res.RepoID != "repo-a" {
		t.Errorf("RepoID = %q, want %q", res.RepoID, "repo-a")
	}
}

// should return Cancelled with toast when no repos registered.
func TestResolveRepoAndRoute_NoReposRegistered(t *testing.T) {
	t.Parallel()
	m := makeResolveRepoModel(t, "PROJ-1", nil, nil) // no assigned, no config repos
	m2, res := resolveRepoAndRoute(m, "PROJ-1")
	if !res.Cancelled {
		t.Error("expected Cancelled when no repos registered")
	}
	// Toast should have been pushed.
	if len(m2.toasts) == 0 {
		t.Error("expected toast to be queued")
	}
}

// should open picker when 0 assigned and repos exist (first-touch).
func TestResolveRepoAndRoute_ZeroAssignedOpensPicker(t *testing.T) {
	t.Parallel()
	m := makeResolveRepoModel(t, "PROJ-1", nil, map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "A"},
	})
	m2, res := resolveRepoAndRoute(m, "PROJ-1")
	if res.Cancelled {
		t.Error("expected not cancelled")
	}
	if !res.PickerOpened {
		t.Error("expected PickerOpened for first-touch")
	}
	if m2.mode != ModeRepoPicker {
		t.Errorf("mode = %v, want ModeRepoPicker", m2.mode)
	}
	if m2.repoPicker == nil {
		t.Error("repoPicker should be non-nil")
	}
	if !m2.repoPicker.firstTouch {
		t.Error("expected firstTouch = true")
	}
}

// should open picker restricted to assigned repos when 2+ assigned.
func TestResolveRepoAndRoute_MultiAssignedOpensPicker(t *testing.T) {
	t.Parallel()
	m := makeResolveRepoModel(t, "PROJ-1", []string{"repo-a", "repo-b"}, map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "A"},
		"repo-b": {ID: "repo-b", Name: "B"},
	})
	m2, res := resolveRepoAndRoute(m, "PROJ-1")
	if res.Cancelled {
		t.Error("expected not cancelled for multi-repo")
	}
	if !res.PickerOpened {
		t.Error("expected PickerOpened for multi-repo")
	}
	if m2.repoPicker == nil {
		t.Fatal("repoPicker should be non-nil")
	}
	if m2.repoPicker.firstTouch {
		t.Error("firstTouch should be false for multi-repo picker")
	}
	if len(m2.repoPicker.rows) != 2 {
		t.Errorf("rows = %d, want 2", len(m2.repoPicker.rows))
	}
}

// should append assigned_repo_ids and return to activation on first-touch select.
func TestHandleRepoPickerMode_FirstTouchSelectAppendsAssignment(t *testing.T) {
	t.Parallel()
	m := makeResolveRepoModel(t, "PROJ-1", nil, map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "A"},
	})
	m.mode = ModeRepoPicker
	m.repoPicker = newRepoPickerState(m.cfg, "PROJ-1", nil, true)
	// cursor=0 = repo-a; press Enter.
	m, _ = m.handleRepoPickerMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyPickerFocus)})
	// repoPicker should be cleared.
	if m.repoPicker != nil {
		t.Error("repoPicker should be nil after selection")
	}
	// assigned_repo_ids should contain repo-a.
	snap, _ := m.store.Snapshot()
	assigned := state.AssignedRepoIDs(snap, "PROJ-1")
	found := false
	for _, id := range assigned {
		if id == "repo-a" {
			found = true
		}
	}
	if !found {
		t.Errorf("repo-a should be in assigned_repo_ids after first-touch select, got %v", assigned)
	}
}

// should return Cancelled with toast when stale id is selected (E-MR1).
func TestHandleRepoPickerMode_StaleIdCancelsWithToast(t *testing.T) {
	t.Parallel()
	m := makeResolveRepoModel(t, "PROJ-1", []string{"ghost"}, map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "A"},
	})
	m.mode = ModeRepoPicker
	// Multi-repo picker with ghost (stale).
	m.repoPicker = newRepoPickerState(m.cfg, "PROJ-1", []string{"ghost"}, false)

	m, _ = m.handleRepoPickerMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyPickerFocus)})
	// Should return to Normal and have a toast.
	if m.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal after stale select", m.mode)
	}
	if len(m.toasts) == 0 {
		t.Error("expected toast for stale repo selection")
	}
	if !containsStr(m.toasts[0].msg, "not registered") {
		t.Errorf("toast msg should mention 'not registered', got %q", m.toasts[0].msg)
	}
}

// Esc on repo picker returns to Normal with no side effects.
func TestHandleRepoPickerMode_EscCancels(t *testing.T) {
	t.Parallel()
	m := makeResolveRepoModel(t, "PROJ-1", nil, map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "A"},
	})
	m.mode = ModeRepoPicker
	m.repoPicker = newRepoPickerState(m.cfg, "PROJ-1", nil, true)
	m, _ = m.handleRepoPickerMode(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal after Esc", m.mode)
	}
	if m.repoPicker != nil {
		t.Error("repoPicker should be nil after Esc")
	}
	// No store mutations.
	snap, _ := m.store.Snapshot()
	if assigned := state.AssignedRepoIDs(snap, "PROJ-1"); len(assigned) != 0 {
		t.Errorf("expected no assignment after Esc, got %v", assigned)
	}
}
