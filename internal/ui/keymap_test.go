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

func TestHandleApproachNameMode_NormalCommitsSelectedTicket(t *testing.T) {
	t.Parallel()
	m := makeTestModelForKeymap(t)
	m = seedTickets(t, m, map[string]state.TicketState{
		"PROJ-1": {Key: "PROJ-1", AssignedRepoIDs: []string{"repo-a"}},
	})
	m.selectedKey = "PROJ-1"
	m.mode = ModeApproachName
	m.previousMode = ModeNormal
	m.approachNameInput.SetValue("side-quest")

	next, cmd := m.handleApproachNameMode(tea.KeyMsg{Type: tea.KeyEnter})
	if next.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", next.mode)
	}
	if cmd == nil {
		t.Fatal("expected activation cmd for selected ticket")
	}
	if !next.activatingTickets["PROJ-1"] {
		t.Error("selected ticket should be marked activating")
	}
}

func TestHandlePickerMode_NewApproachPreservesContext(t *testing.T) {
	t.Parallel()
	m := makeTestModelForKeymap(t)
	m.mode = ModePicker
	m.pickerState = &pickerState{ticketID: "PROJ-1", repoID: "repo-a"}

	next, _ := m.handlePickerMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyPickerNew)})
	if next.mode != ModeApproachName {
		t.Errorf("mode = %v, want ModeApproachName", next.mode)
	}
	if next.pickerState == nil {
		t.Fatal("pickerState should be preserved while naming the approach")
	}

	next.approachNameInput.SetValue("experiment")
	committed, cmd := next.handleApproachNameMode(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected activation cmd for picker ticket/repo")
	}
	if committed.pickerState != nil {
		t.Error("pickerState should be cleared after commit")
	}
	if !committed.activatingTickets["PROJ-1"] {
		t.Error("picker ticket should be marked activating")
	}
}

func TestHandleApproachNameMode_CarriesNameThroughRepoPicker(t *testing.T) {
	t.Parallel()
	m := makeResolveRepoModel(t, "PROJ-1", nil, map[string]config.RepoEntry{
		"repo-a": {ID: "repo-a", Name: "Repo A"},
	})
	m.selectedKey = "PROJ-1"
	m.mode = ModeApproachName
	m.previousMode = ModeNormal
	m.approachNameInput.SetValue("fresh")

	next, cmd := m.handleApproachNameMode(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("repo picker should defer activation until a repo is selected")
	}
	if next.mode != ModeRepoPicker {
		t.Errorf("mode = %v, want ModeRepoPicker", next.mode)
	}
	if next.repoPicker == nil {
		t.Fatal("repoPicker should be opened")
	}
	if next.repoPicker.pendingApproach != "fresh" {
		t.Errorf("pendingApproach = %q, want fresh", next.repoPicker.pendingApproach)
	}

	committed, cmd := next.handleRepoPickerMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyPickerFocus)})
	if cmd == nil {
		t.Fatal("expected activation cmd after selecting repo")
	}
	if !committed.activatingTickets["PROJ-1"] {
		t.Error("ticket should be marked activating after repo selection")
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
