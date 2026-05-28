package ui

import (
	"fmt"

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
		m.mousePressCount++
		return m.handleMousePress(msg)
	case tea.MouseActionMotion:
		m.mouseMotionCount++
		return m.handleMouseMotion(msg)
	case tea.MouseActionRelease:
		m.mouseReleaseCount++
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
	// Reconcile zOrder against the current snapshot before hit-testing.
	// View() reconciles via a value-receiver copy that never reaches Update; without this
	// the first MouseMsg sees an empty zOrder and no zone is hit, breaking click and drag.
	if snap, _ := m.store.Snapshot(); snap != nil {
		m.reconcileZOrder(snap)
	}
	// Find the topmost card that contains the cursor.
	// Walk zOrder from the end (topmost) and check zone bounds.
	key := ""
	var dbg string
	for i := len(m.zOrder) - 1; i >= 0; i-- {
		k := m.zOrder[i]
		if zone.DefaultManager != nil {
			info := zone.Get(k)
			if info == nil {
				dbg += fmt.Sprintf(" %s=nil", k)
				continue
			}
			if info.IsZero() {
				dbg += fmt.Sprintf(" %s=zero", k)
				continue
			}
			dbg += fmt.Sprintf(" %s=(%d,%d)-(%d,%d)", k, info.StartX, info.StartY, info.EndX, info.EndY)
			if info.InBounds(msg) {
				key = k
				break
			}
		}
	}
	if key == "" {
		m.dragDebug = fmt.Sprintf("press@(%d,%d) MISS zones:%s", msg.X, msg.Y, dbg)
		return m, nil
	}
	// Seed live-drag position from the card's current (x, y) so the canvas can
	// render it in place until motion arrives.
	startX, startY := 0, 0
	if snap, _ := m.store.Snapshot(); snap != nil {
		if t, ok := snap.Tickets[key]; ok {
			startX, startY = t.X, t.Y
		}
	}
	m.dragging = &dragState{
		key:      key,
		pressX:   msg.X,
		pressY:   msg.Y,
		motion:   false,
		currentX: startX,
		currentY: startY,
	}
	m.dragDebug = fmt.Sprintf("press@(%d,%d) HIT=%s", msg.X, msg.Y, key)
	return m, nil
}

// handleMouseMotion updates drag state when cursor moves while a drag is in flight.
// The displayed position is updated in transient state; it is NOT persisted until release.
// A copy of dragState is made to avoid aliased mutation between old and new Model values.
func (m Model) handleMouseMotion(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.dragging == nil {
		return m, nil
	}
	if msg.X == m.dragging.pressX && msg.Y == m.dragging.pressY {
		return m, nil
	}
	// Copy dragState to avoid aliased writes across old/new BubbleTea Model values.
	ds := *m.dragging
	ds.motion = true
	// Recompute live position from the card's original (x, y) at press time.
	// Use the snapshot to find the press-time origin; if the snapshot doesn't have
	// the key (rare race), fall back to the previously computed currentX/Y.
	if snap, _ := m.store.Snapshot(); snap != nil {
		if t, ok := snap.Tickets[ds.key]; ok {
			nx, ny := uispatial.ApplyDelta(t.X, t.Y, ds.pressX, ds.pressY, msg.X, msg.Y)
			nx, ny = uispatial.Clamp(nx, ny, cardWidth(m.width), cardHeight(), m.width, m.height)
			ds.currentX, ds.currentY = nx, ny
		}
	}
	m.dragging = &ds
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
		m.dragDebug = fmt.Sprintf("release CLICK key=%s", key)
		return m.tryActivate(key)
	}

	// Drag-release: compute clamped position and persist via store.Mutate.
	snap := m.snapshot
	if snap == nil {
		m.dragDebug = "release DRAG snap=nil"
		return m, nil
	}
	card, ok := snap.Tickets[key]
	if !ok {
		m.dragDebug = fmt.Sprintf("release DRAG missing key=%s", key)
		return m, nil
	}
	newX, newY := uispatial.ApplyDelta(card.X, card.Y, pressX, pressY, msg.X, msg.Y)
	newX, newY = uispatial.Clamp(newX, newY, cardWidth(m.width), cardHeight(), m.width, m.height)

	if err := m.store.Mutate(func(s *state.State) error {
		state.SetTicketPosition(s, key, newX, newY)
		return nil
	}); err != nil {
		m.dragDebug = "release DRAG mutate-err: " + err.Error()
		return m.pushToast("drag failed: " + err.Error())
	}
	snap2, rev := m.store.Snapshot()
	m.snapshot = snap2
	m.snapshotRev = rev

	m.dragDebug = fmt.Sprintf("release DRAG key=%s (%d,%d)->(%d,%d)", key, card.X, card.Y, newX, newY)
	return m, nil
}

// cardHeight returns the default card height for clamp calculations.
// Post-it cards are approximately 5 lines tall (border + header + title + labels + margin).
func cardHeight() int {
	return 5
}
