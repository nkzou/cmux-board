package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nkzou/cmux-board/internal/state"
)

// TestMouse_RealZoneDragSetsDragging probes whether bubblezone hit-testing actually
// works end-to-end: render canvas, scan zones, press inside a known-positioned card.
// If this test passes the production path should work too.
func TestMouse_RealZoneDragSetsDragging(t *testing.T) {
	tickets := map[string]state.TicketState{
		"T-1": {Key: "T-1", Summary: "Test Card", X: 10, Y: 5, Source: "local", LocalStatus: "Open"},
	}
	m := buildMouseModel(t, tickets, []string{"T-1"})

	raw := m.renderPostitCanvas()
	scanZone(raw)

	// Card was placed at X=10, Y=5. Press at (15, 6) which should be inside the card.
	press := tea.MouseMsg{X: 15, Y: 6, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	next, _ := m.handleMouseMsg(press)
	m1 := next.(Model)

	if m1.dragging == nil {
		t.Fatalf("dragging should be set after press inside card zone; raw output length=%d", len(raw))
	}
	if m1.dragging.key != "T-1" {
		t.Errorf("dragging.key = %q, want T-1", m1.dragging.key)
	}
}
