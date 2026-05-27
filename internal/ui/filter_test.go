package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/state"
)

// ticketsMap builds a map[string]*state.TicketState from key/summary pairs.
func ticketsMap(pairs ...string) map[string]*state.TicketState {
	m := make(map[string]*state.TicketState, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		key := pairs[i]
		m[key] = &state.TicketState{Key: key, Summary: pairs[i+1]}
	}
	return m
}

// should return all tickets when query is empty.
func TestFilterTickets_EmptyQueryReturnsAll(t *testing.T) {
	t.Parallel()
	tickets := ticketsMap("PROJ-1", "Build UI", "PROJ-2", "Fix bug")
	result := filterTickets(tickets, "")
	if len(result) != 2 {
		t.Errorf("len(result) = %d, want 2", len(result))
	}
}

// should match ticket key case-insensitively.
func TestFilterTickets_CaseInsensitiveKeyMatch(t *testing.T) {
	t.Parallel()
	tickets := ticketsMap("PROJ-42", "Some feature", "PROJ-1", "Other")
	result := filterTickets(tickets, "proj-4")
	if len(result) != 1 {
		t.Errorf("len(result) = %d, want 1 for key match 'proj-4'", len(result))
	}
	if result[0] != "PROJ-42" {
		t.Errorf("result[0] = %q, want %q", result[0], "PROJ-42")
	}
}

// should match ticket summary substring case-insensitively.
func TestFilterTickets_SummarySubstringMatch(t *testing.T) {
	t.Parallel()
	tickets := ticketsMap("PROJ-1", "Fix the Login Button", "PROJ-2", "Refactor DB")
	result := filterTickets(tickets, "login")
	if len(result) != 1 {
		t.Errorf("len(result) = %d, want 1 for summary match 'login'", len(result))
	}
	if result[0] != "PROJ-1" {
		t.Errorf("result[0] = %q, want %q", result[0], "PROJ-1")
	}
}

// should return empty slice when no match.
func TestFilterTickets_NoMatch(t *testing.T) {
	t.Parallel()
	tickets := ticketsMap("PROJ-1", "Build UI", "PROJ-2", "Fix bug")
	result := filterTickets(tickets, "xyz-9999")
	if len(result) != 0 {
		t.Errorf("len(result) = %d, want 0 for no match", len(result))
	}
}

// should clear filter on Esc and return to Normal mode.
func TestHandleFilterMode_EscClearsAndReturnsNormal(t *testing.T) {
	t.Parallel()
	m := makeTestModel(t)
	m.mode = ModeFilter
	m.filterInput.Focus()
	// Type some characters first (simulate typing "abc").
	m, _ = m.handleFilterMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m, _ = m.handleFilterMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	// Esc should clear.
	m, _ = m.handleFilterMode(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal after Esc", m.mode)
	}
	if m.filterQuery != "" {
		t.Errorf("filterQuery = %q, want empty after Esc", m.filterQuery)
	}
}
