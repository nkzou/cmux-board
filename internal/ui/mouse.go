package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/nkzou/cmux-board/internal/state"
	uispatial "github.com/nkzou/cmux-board/internal/ui/spatial"
)

// handleMouseMsg routes tea.MouseMsg to the appropriate press/motion/release handler.
//
// Pre-implementation verification (Review F-13):
//
//	tea.MouseMsg (= tea.MouseEvent) fields: X int, Y int, Action MouseAction, Button MouseButton.
//	Constants (bubbletea v1.3.4):
//	  tea.MouseActionPress, tea.MouseActionMotion, tea.MouseActionRelease
//	  tea.MouseButtonLeft
//
// All state writes go through store.Mutate (Mandatory Invariant 1).
// tryActivate is dispatched on click-release (no motion); position is persisted on drag-release.
func (m Model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Action {
	case tea.MouseActionPress:
		return m.handleMousePress(msg)
	case tea.MouseActionMotion:
		return m.handleMouseMotion(msg)
	case tea.MouseActionRelease:
		return m.handleMouseRelease(msg)
	}
	return m, nil
}

// handleMousePress records the pressed ticket via zone hit-test.
// If no ticket is hit, drag state is not set (no-op).
func (m Model) handleMousePress(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	// Find the topmost card that contains the cursor.
	// Walk zOrder from the end (topmost) and check zone bounds.
	key := ""
	for i := len(m.zOrder) - 1; i >= 0; i-- {
		k := m.zOrder[i]
		if zone.DefaultManager != nil {
			info := zone.Get(k)
			if info != nil && !info.IsZero() && info.InBounds(msg) {
				key = k
				break
			}
		}
	}
	if key == "" {
		return m, nil
	}
	m.dragging = &dragState{
		key:    key,
		pressX: msg.X,
		pressY: msg.Y,
		motion: false,
	}
	return m, nil
}

// handleMouseMotion updates drag state when cursor moves while a drag is in flight.
// The displayed position is updated in transient state; it is NOT persisted until release.
// A copy of dragState is made to avoid aliased mutation between old and new Model values.
func (m Model) handleMouseMotion(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.dragging == nil {
		return m, nil
	}
	if msg.X != m.dragging.pressX || msg.Y != m.dragging.pressY {
		// Copy dragState to avoid aliased writes across old/new BubbleTea Model values.
		ds := *m.dragging
		ds.motion = true
		m.dragging = &ds
	}
	return m, nil
}

// handleMouseRelease processes mouse-button release.
//   - If no drag is in flight: no-op.
//   - If motion == false (click): dispatch tryActivate and promote to top of zOrder.
//   - If motion == true (drag): persist clamped (x, y) via store.Mutate and promote to top.
func (m Model) handleMouseRelease(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.dragging == nil {
		return m, nil
	}

	key := m.dragging.key
	motion := m.dragging.motion
	pressX := m.dragging.pressX
	pressY := m.dragging.pressY
	m.dragging = nil

	// Promote key to top of zOrder regardless of click vs. drag (Review F-06).
	m.zOrder = uispatial.ZOrder(m.zOrder, key)

	if !motion {
		// Click-release: select and activate.
		m.selectedKey = key
		return m.tryActivate(key)
	}

	// Drag-release: compute clamped position and persist via store.Mutate.
	snap := m.snapshot
	if snap == nil {
		return m, nil
	}
	card, ok := snap.Tickets[key]
	if !ok {
		return m, nil
	}
	newX, newY := uispatial.ApplyDelta(card.X, card.Y, pressX, pressY, msg.X, msg.Y)
	newX, newY = uispatial.Clamp(newX, newY, cardWidth(m.width), cardHeight(), m.width, m.height)

	if err := m.store.Mutate(func(s *state.State) error {
		state.SetTicketPosition(s, key, newX, newY)
		return nil
	}); err != nil {
		return m.pushToast("drag failed: " + err.Error())
	}
	snap2, rev := m.store.Snapshot()
	m.snapshot = snap2
	m.snapshotRev = rev

	return m, nil
}

// cardHeight returns the default card height for clamp calculations.
// Post-it cards are approximately 5 lines tall (border + header + title + labels + margin).
func cardHeight() int {
	return 5
}
