package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/state"
)

// seedTickets populates the model's store and refreshes the snapshot with the given tickets.
func seedTickets(t *testing.T, m Model, tickets map[string]state.TicketState) Model {
	t.Helper()
	if err := m.store.Mutate(func(s *state.State) error {
		for k, v := range tickets {
			s.Tickets[k] = v
		}
		return nil
	}); err != nil {
		t.Fatalf("seed Mutate: %v", err)
	}
	snap, rev := m.store.Snapshot()
	m.snapshot = snap
	m.snapshotRev = rev
	return m
}

// TestNavigate_ArrowRight_SelectsNearestEastNeighbor verifies that pressing the right
// arrow key moves m.selectedKey to the nearest ticket to the east.
func TestNavigate_ArrowRight_SelectsNearestEastNeighbor(t *testing.T) {
	t.Parallel()
	m := makeTestModelForKeymap(t)
	m = seedTickets(t, m, map[string]state.TicketState{
		"A": {Key: "A", Source: "local", X: 0, Y: 0},
		"B": {Key: "B", Source: "local", X: 20, Y: 0},  // nearest east of A
		"C": {Key: "C", Source: "local", X: 40, Y: 0},  // farther east
		"D": {Key: "D", Source: "local", X: 0, Y: 10},  // south, not east
	})
	m.zOrder = []string{"A", "B", "C", "D"}
	m.selectedKey = "A"

	next, _ := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyRight)})
	if next.selectedKey != "B" {
		t.Errorf("selectedKey = %q, want %q", next.selectedKey, "B")
	}
}

// TestNavigate_TabCyclesZOrder verifies that Tab moves m.selectedKey to the next entry
// in m.zOrder with wrap-around.
func TestNavigate_TabCyclesZOrder(t *testing.T) {
	t.Parallel()
	m := makeTestModelForKeymap(t)
	m = seedTickets(t, m, map[string]state.TicketState{
		"A": {Key: "A", Source: "local"},
		"B": {Key: "B", Source: "local"},
		"C": {Key: "C", Source: "local"},
	})
	m.zOrder = []string{"A", "B", "C"}
	m.selectedKey = "A"

	// Tab once: A -> B
	next, _ := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyTab})
	if next.selectedKey != "B" {
		t.Errorf("after Tab: selectedKey = %q, want %q", next.selectedKey, "B")
	}

	// Tab twice: B -> C
	next2, _ := next.handleNormalMode(tea.KeyMsg{Type: tea.KeyTab})
	if next2.selectedKey != "C" {
		t.Errorf("after Tab Tab: selectedKey = %q, want %q", next2.selectedKey, "C")
	}

	// Tab wrap: C -> A
	next3, _ := next2.handleNormalMode(tea.KeyMsg{Type: tea.KeyTab})
	if next3.selectedKey != "A" {
		t.Errorf("after Tab Tab Tab (wrap): selectedKey = %q, want %q", next3.selectedKey, "A")
	}
}

// TestNavigate_ShiftTabCyclesZOrderReverse verifies Shift-Tab cycles backwards.
func TestNavigate_ShiftTabCyclesZOrderReverse(t *testing.T) {
	t.Parallel()
	m := makeTestModelForKeymap(t)
	m = seedTickets(t, m, map[string]state.TicketState{
		"A": {Key: "A", Source: "local"},
		"B": {Key: "B", Source: "local"},
		"C": {Key: "C", Source: "local"},
	})
	m.zOrder = []string{"A", "B", "C"}
	m.selectedKey = "A"

	// Shift-Tab from A: wraps to C
	next, _ := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyShiftTab})
	if next.selectedKey != "C" {
		t.Errorf("after Shift-Tab: selectedKey = %q, want %q", next.selectedKey, "C")
	}
}

// TestKey_S_CyclesStatus_LeavesXYUnchanged verifies F6: cycling status does not move the ticket.
func TestKey_S_CyclesStatus_LeavesXYUnchanged(t *testing.T) {
	t.Parallel()
	m := makeTestModelForKeymap(t)
	m = seedTickets(t, m, map[string]state.TicketState{
		"A": {Key: "A", Source: "local", LocalStatus: "Open", X: 5, Y: 10},
	})
	m.zOrder = []string{"A"}
	m.selectedKey = "A"

	_, _ = m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyCycleStatus)})

	snap, _ := m.store.Snapshot()
	ts := snap.Tickets["A"]
	if ts.LocalStatus == "Open" {
		t.Error("LocalStatus should have changed from Open")
	}
	if ts.X != 5 || ts.Y != 10 {
		t.Errorf("position changed: got (%d, %d), want (5, 10)", ts.X, ts.Y)
	}
}

// TestKey_X_RemovesSelected verifies that pressing x removes the selected ticket.
func TestKey_X_RemovesSelected(t *testing.T) {
	t.Parallel()
	m := makeTestModelForKeymap(t)
	m = seedTickets(t, m, map[string]state.TicketState{
		"A": {Key: "A", Source: "local"},
		"B": {Key: "B", Source: "local"},
	})
	m.zOrder = []string{"A", "B"}
	m.selectedKey = "A"

	_, _ = m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyRemoveTicket)})

	snap, _ := m.store.Snapshot()
	if _, exists := snap.Tickets["A"]; exists {
		t.Error("ticket A should have been removed")
	}
	if _, exists := snap.Tickets["B"]; !exists {
		t.Error("ticket B should still exist")
	}
}

// TestKey_I_EntersImportMode verifies that 'i' transitions to ModeImportInput.
func TestKey_I_EntersImportMode(t *testing.T) {
	t.Parallel()
	m := makeTestModelForKeymap(t)
	m = seedTickets(t, m, map[string]state.TicketState{
		"A": {Key: "A", Source: "local"},
	})
	m.zOrder = []string{"A"}
	m.selectedKey = "A"

	next, _ := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyImportJira)})
	if next.mode != ModeImportInput {
		t.Errorf("mode = %v, want ModeImportInput", next.mode)
	}
}

// TestKey_C_EntersCreateMode verifies that 'c' transitions to ModeCreateInput.
func TestKey_C_EntersCreateMode(t *testing.T) {
	t.Parallel()
	m := makeTestModelForKeymap(t)
	m = seedTickets(t, m, map[string]state.TicketState{
		"A": {Key: "A", Source: "local"},
	})
	m.zOrder = []string{"A"}
	m.selectedKey = "A"

	next, _ := m.handleNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyCreateLocal)})
	if next.mode != ModeCreateInput {
		t.Errorf("mode = %v, want ModeCreateInput", next.mode)
	}
}
