package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/state"
)

// filterTickets returns the subset of ticketIDs whose key or summary contains query
// (case-insensitive). If query is empty, all IDs are returned in their original order.
func filterTickets(tickets map[string]*state.TicketState, query string) []string {
	if query == "" {
		ids := make([]string, 0, len(tickets))
		for id := range tickets {
			ids = append(ids, id)
		}
		return ids
	}
	lower := strings.ToLower(query)
	var out []string
	for id, t := range tickets {
		if t != nil && ticketMatchesFilter(*t, lower) {
			out = append(out, id)
		}
	}
	return out
}

// filterTicketsByState is the same as filterTickets but accepts map[string]state.TicketState
// (value, not pointer), matching the State.Tickets field type.
func filterTicketsByState(tickets map[string]state.TicketState, query string) []string {
	if query == "" {
		ids := make([]string, 0, len(tickets))
		for id := range tickets {
			ids = append(ids, id)
		}
		return ids
	}
	lower := strings.ToLower(query)
	var out []string
	for id, t := range tickets {
		if ticketMatchesFilter(t, lower) {
			out = append(out, id)
		}
	}
	return out
}

// ticketMatchesFilter reports whether a ticket should be emphasized for query.
// query may already be lower-cased by callers.
func ticketMatchesFilter(t state.TicketState, query string) bool {
	if query == "" {
		return true
	}
	lower := strings.ToLower(query)
	return strings.Contains(strings.ToLower(t.Key), lower) ||
		strings.Contains(strings.ToLower(t.Summary), lower)
}

func (m Model) ticketFilteredOut(t state.TicketState) bool {
	return m.filterQuery != "" && !ticketMatchesFilter(t, m.filterQuery)
}

// handleFilterMode handles key events when mode == ModeFilter.
// Printable characters are delegated to filterInput. Esc or empty+Enter exits.
func (m Model) handleFilterMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case KeyEsc:
		m.filterQuery = ""
		m.filterInput.SetValue("")
		m.filterInput.Blur()
		m.mode = ModeNormal
		return m, nil
	case "enter":
		if m.filterInput.Value() == "" {
			m.filterQuery = ""
			m.filterInput.Blur()
			m.mode = ModeNormal
			return m, nil
		}
		m.filterQuery = m.filterInput.Value()
		return m, nil
	default:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.filterQuery = m.filterInput.Value()
		return m, cmd
	}
}
