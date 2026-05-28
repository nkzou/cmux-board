package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

// makePickerTestModel builds a Model with the given activations seeded in the store.
func makePickerTestModel(t *testing.T, activations []state.ActivationEntry, repos map[string]config.RepoEntry) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	if len(activations) > 0 {
		if err := store.Mutate(func(s *state.State) error {
			for _, a := range activations {
				s.Activations[a.TicketID] = append(s.Activations[a.TicketID], a)
			}
			return nil
		}); err != nil {
			t.Fatalf("store.Mutate: %v", err)
		}
	}
	cfg := &config.Config{
		SchemaVersion: config.SchemaVersionCurrent,
		Repos:         repos,
	}
	return NewModel(cfg, store)
}

func makeEntry(ticketID, repoID, activationID, step string, complete bool) state.ActivationEntry {
	return state.ActivationEntry{
		ActivationID: activationID,
		ActIDShort:   activationID[:8],
		RepoID:       repoID,
		TicketID:     ticketID,
		ApproachName: "approach-" + activationID[:4],
		CmuxName:     ticketID + " [" + activationID[:8] + "]",
		Step:         step,
		Complete:     complete,
		CreatedAt:    time.Now(),
	}
}

// TestNewPickerState_NilForZeroEntries: empty activations → returns nil.
func TestNewPickerState_NilForZeroEntries(t *testing.T) {
	t.Parallel()
	snap := &state.State{
		SchemaVersion: state.SchemaVersionCurrent,
		Activations:   make(map[string][]state.ActivationEntry),
	}
	ps := newPickerState(snap, "PROJ-1", "repo-a")
	if ps != nil {
		t.Errorf("expected nil for 0 entries, got non-nil")
	}
}

// TestNewPickerState_NilForOneEntry: 1 activation → returns nil.
func TestNewPickerState_NilForOneEntry(t *testing.T) {
	t.Parallel()
	snap := &state.State{
		SchemaVersion: state.SchemaVersionCurrent,
		Activations: map[string][]state.ActivationEntry{
			"PROJ-1": {makeEntry("PROJ-1", "repo-a", "01AAAAAAAAAAAAAAAAAAAAAA", state.StepCmuxCreated, true)},
		},
	}
	ps := newPickerState(snap, "PROJ-1", "repo-a")
	if ps != nil {
		t.Errorf("expected nil for 1 entry, got non-nil")
	}
}

// TestNewPickerState_NonNilForTwo: 2 activations → returns non-nil with cursor 0.
func TestNewPickerState_NonNilForTwo(t *testing.T) {
	t.Parallel()
	snap := &state.State{
		SchemaVersion: state.SchemaVersionCurrent,
		Activations: map[string][]state.ActivationEntry{
			"PROJ-1": {
				makeEntry("PROJ-1", "repo-a", "01AAAAAAAAAAAAAAAAAAAAAA", state.StepCmuxCreated, true),
				makeEntry("PROJ-1", "repo-a", "01BBBBBBBBBBBBBBBBBBBBBB", state.StepCmuxCreated, true),
			},
		},
	}
	ps := newPickerState(snap, "PROJ-1", "repo-a")
	if ps == nil {
		t.Fatal("expected non-nil for 2 entries, got nil")
	}
	if ps.cursorIdx != 0 {
		t.Errorf("cursorIdx = %d, want 0", ps.cursorIdx)
	}
	if len(ps.entries) != 2 {
		t.Errorf("len(entries) = %d, want 2", len(ps.entries))
	}
}

// TestRenderPicker_OrphanGlyph: entry with claude_orphan: true → rendered line contains [orphan].
func TestRenderPicker_OrphanGlyph(t *testing.T) {
	t.Parallel()
	entry := makeEntry("PROJ-1", "repo-a", "01AAAAAAAAAAAAAAAAAAAAAA", state.StepCmuxCreated, true)
	entry.ClaudeOrphan = true
	entry2 := makeEntry("PROJ-1", "repo-a", "01BBBBBBBBBBBBBBBBBBBBBB", state.StepCmuxCreated, true)
	m := makePickerTestModel(t,
		[]state.ActivationEntry{entry, entry2},
		map[string]config.RepoEntry{"repo-a": {ID: "repo-a", Name: "Repo A"}},
	)
	snap, _ := m.store.Snapshot()
	m.snapshot = snap
	m.pickerState = newPickerState(snap, "PROJ-1", "repo-a")
	m.width = 100
	m.height = 40

	rendered := m.renderPicker()
	if !containsStr(rendered, "[orphan]") {
		t.Errorf("expected [orphan] glyph in rendered output:\n%s", rendered)
	}
}

// TestRenderPicker_NoTabGlyph: entry with cmux_orphan: true → line contains [no-tab].
func TestRenderPicker_NoTabGlyph(t *testing.T) {
	t.Parallel()
	entry := makeEntry("PROJ-1", "repo-a", "01AAAAAAAAAAAAAAAAAAAAAA", state.StepCmuxCreated, true)
	entry.CmuxOrphan = true
	entry2 := makeEntry("PROJ-1", "repo-a", "01BBBBBBBBBBBBBBBBBBBBBB", state.StepCmuxCreated, true)
	m := makePickerTestModel(t,
		[]state.ActivationEntry{entry, entry2},
		map[string]config.RepoEntry{"repo-a": {ID: "repo-a", Name: "Repo A"}},
	)
	snap, _ := m.store.Snapshot()
	m.snapshot = snap
	m.pickerState = newPickerState(snap, "PROJ-1", "repo-a")
	m.width = 100
	m.height = 40

	rendered := m.renderPicker()
	if !containsStr(rendered, "[no-tab]") {
		t.Errorf("expected [no-tab] glyph in rendered output:\n%s", rendered)
	}
}

// TestRenderPicker_UnknownRepoGlyph: repo_id missing from cfg.Repos → [?].
func TestRenderPicker_UnknownRepoGlyph(t *testing.T) {
	t.Parallel()
	entry := makeEntry("PROJ-1", "ghost-repo", "01AAAAAAAAAAAAAAAAAAAAAA", state.StepCmuxCreated, true)
	entry2 := makeEntry("PROJ-1", "ghost-repo", "01BBBBBBBBBBBBBBBBBBBBBB", state.StepCmuxCreated, true)
	// cfg.Repos does NOT contain "ghost-repo"
	m := makePickerTestModel(t,
		[]state.ActivationEntry{entry, entry2},
		map[string]config.RepoEntry{"repo-a": {ID: "repo-a", Name: "Repo A"}},
	)
	snap, _ := m.store.Snapshot()
	m.snapshot = snap
	m.pickerState = newPickerState(snap, "PROJ-1", "ghost-repo")
	m.width = 100
	m.height = 40

	rendered := m.renderPicker()
	if !containsStr(rendered, "[?]") {
		t.Errorf("expected [?] glyph for unknown repo in rendered output:\n%s", rendered)
	}
}

// TestRenderPicker_IncompleteGlyph: complete: false, step: "claude_started" → [incomplete 2/3].
func TestRenderPicker_IncompleteGlyph(t *testing.T) {
	t.Parallel()
	entry := makeEntry("PROJ-1", "repo-a", "01AAAAAAAAAAAAAAAAAAAAAA", state.StepClaudeStarted, false)
	entry2 := makeEntry("PROJ-1", "repo-a", "01BBBBBBBBBBBBBBBBBBBBBB", state.StepCmuxCreated, true)
	m := makePickerTestModel(t,
		[]state.ActivationEntry{entry, entry2},
		map[string]config.RepoEntry{"repo-a": {ID: "repo-a", Name: "Repo A"}},
	)
	snap, _ := m.store.Snapshot()
	m.snapshot = snap
	m.pickerState = newPickerState(snap, "PROJ-1", "repo-a")
	m.width = 100
	m.height = 40

	rendered := m.renderPicker()
	if !containsStr(rendered, "[incomplete 2/3]") {
		t.Errorf("expected [incomplete 2/3] glyph in rendered output:\n%s", rendered)
	}
}

// TestPickerDelete_RemovesEntry: d with entries → after all deleted, pickerState nil and mode Normal.
func TestPickerDelete_RemovesEntry(t *testing.T) {
	t.Parallel()
	entry := makeEntry("PROJ-1", "repo-a", "01AAAAAAAAAAAAAAAAAAAAAA", state.StepCmuxCreated, true)
	entry2 := makeEntry("PROJ-1", "repo-a", "01BBBBBBBBBBBBBBBBBBBBBB", state.StepCmuxCreated, true)
	m := makePickerTestModel(t,
		[]state.ActivationEntry{entry, entry2},
		map[string]config.RepoEntry{"repo-a": {ID: "repo-a", Name: "Repo A"}},
	)
	snap, _ := m.store.Snapshot()
	m.snapshot = snap
	m.pickerState = newPickerState(snap, "PROJ-1", "repo-a")
	m.mode = ModePicker

	// Delete first entry (cursorIdx=0).
	m, _ = m.handlePickerMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyPickerDelete)})
	// One entry remains; picker should still be open.
	if m.pickerState == nil {
		t.Fatal("expected picker to remain open after deleting 1 of 2 entries")
	}
	if len(m.pickerState.entries) != 1 {
		t.Errorf("len(entries) = %d, want 1 after first delete", len(m.pickerState.entries))
	}

	// Delete the remaining entry.
	m, _ = m.handlePickerMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyPickerDelete)})
	if m.pickerState != nil {
		t.Errorf("expected pickerState nil after all entries deleted")
	}
	if m.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", m.mode)
	}
}

// TestPickerCursorClamping: ↓ past end → clamped at last.
func TestPickerCursorClamping(t *testing.T) {
	t.Parallel()
	entry := makeEntry("PROJ-1", "repo-a", "01AAAAAAAAAAAAAAAAAAAAAA", state.StepCmuxCreated, true)
	entry2 := makeEntry("PROJ-1", "repo-a", "01BBBBBBBBBBBBBBBBBBBBBB", state.StepCmuxCreated, true)
	m := makePickerTestModel(t,
		[]state.ActivationEntry{entry, entry2},
		map[string]config.RepoEntry{"repo-a": {ID: "repo-a", Name: "Repo A"}},
	)
	snap, _ := m.store.Snapshot()
	m.snapshot = snap
	m.pickerState = newPickerState(snap, "PROJ-1", "repo-a")
	m.mode = ModePicker

	// Press down 5 times; cursor should clamp at 1 (last index).
	for i := 0; i < 5; i++ {
		m, _ = m.handlePickerMode(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.pickerState.cursorIdx != 1 {
		t.Errorf("cursorIdx = %d, want 1 after clamping at end", m.pickerState.cursorIdx)
	}

	// Press up 5 times; cursor should clamp at 0.
	for i := 0; i < 5; i++ {
		m, _ = m.handlePickerMode(tea.KeyMsg{Type: tea.KeyUp})
	}
	if m.pickerState.cursorIdx != 0 {
		t.Errorf("cursorIdx = %d, want 0 after clamping at start", m.pickerState.cursorIdx)
	}
}
