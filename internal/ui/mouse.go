package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

// handleMousePress records the pressed ticket via geometric hit-test.
// If no ticket is hit, drag state is not set (no-op).
func (m Model) handleMousePress(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	// Reconcile zOrder against the current snapshot before hit-testing.
	// View() reconciles via a value-receiver copy that never reaches Update; without this
	// the first MouseMsg sees an empty zOrder and no card is hit, breaking click and drag.
	snap, _ := m.store.Snapshot()
	if snap != nil {
		m.reconcileZOrder(snap)
	}
	key, found, dbg := m.hitTestTicketAt(snap, msg.X, msg.Y)
	if !found {
		m.dragDebug = fmt.Sprintf("press@(%d,%d) MISS boxes:%s", msg.X, msg.Y, dbg)
		return m, nil
	}
	// Seed live-drag position from the card's current (x, y) so the canvas can
	// render it in place until motion arrives.
	startX, startY := 0, 0
	if t, ok := snap.Tickets[key]; ok {
		startX, startY = t.X, t.Y
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

func (m Model) hitTestTicketAt(snap *state.State, screenX, screenY int) (string, bool, string) {
	if snap == nil {
		return "", false, "snap=nil"
	}
	canvasW := m.width
	if canvasW <= 0 {
		canvasW = defaultTerminalWidth
	}
	canvasY := screenY - headerRows
	if screenX < 0 || screenX >= canvasW || canvasY < 0 || canvasY >= m.canvasHeight() {
		return "", false, fmt.Sprintf("outside-canvas canvasY=%d", canvasY)
	}

	boxes := m.ticketHitBoxes(snap)
	key, found := uispatial.HitTest(boxes, m.zOrder, screenX, canvasY)
	return key, found, formatHitBoxes(boxes)
}

func (m Model) ticketHitBoxes(snap *state.State) []uispatial.Positioned {
	boxes := make([]uispatial.Positioned, 0, len(snap.Tickets))
	for _, k := range m.zOrder {
		ticket, ok := snap.Tickets[k]
		if !ok {
			continue
		}
		w, h := m.renderedTicketSize(snap, k, ticket)
		if w <= 0 || h <= 0 {
			continue
		}
		boxes = append(boxes, uispatial.Positioned{
			ID: k,
			X:  ticket.X,
			Y:  ticket.Y,
			W:  w,
			H:  h,
		})
	}
	return boxes
}

func (m Model) renderedTicketSize(snap *state.State, key string, ticket state.TicketState) (int, int) {
	canvasW := m.width
	if canvasW <= 0 {
		canvasW = defaultTerminalWidth
	}
	width := cardWidth(canvasW)
	rendered := renderTicket(renderTicketParams{
		ticket:       ticketStateToUI(ticket, m.activatingTickets[key], state.ActivationCount(snap, key)),
		isSelected:   key == m.selectedKey,
		filteredOut:  m.ticketFilteredOut(ticket),
		width:        width,
		accentColor:  defaultColors().primary,
		colors:       defaultColors(),
		spinnerGlyph: m.spinnerGlyph(),
	})
	lines := strings.Split(rendered, "\n")
	maxW := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > maxW {
			maxW = w
		}
	}
	return maxW, len(lines)
}

func formatHitBoxes(boxes []uispatial.Positioned) string {
	if len(boxes) == 0 {
		return " none"
	}
	var b strings.Builder
	for _, box := range boxes {
		fmt.Fprintf(&b, " %s=(%d,%d)-(%d,%d)", box.ID, box.X, box.Y, box.X+box.W, box.Y+box.H)
	}
	return b.String()
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
			cardW, cardH := m.dragCardSize(snap, ds.key)
			nx, ny = uispatial.Clamp(nx, ny, cardW, cardH, m.canvasWidth(), m.canvasHeight())
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
	cardW, cardH := m.dragCardSize(snap, key)
	newX, newY = uispatial.Clamp(newX, newY, cardW, cardH, m.canvasWidth(), m.canvasHeight())

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

func (m Model) canvasWidth() int {
	if m.width <= 0 {
		return defaultTerminalWidth
	}
	return m.width
}

func (m Model) dragCardSize(snap *state.State, key string) (int, int) {
	if snap != nil {
		if ticket, ok := snap.Tickets[key]; ok {
			w, h := m.renderedTicketSize(snap, key, ticket)
			if w > 0 && h > 0 {
				return w, h
			}
		}
	}
	return cardWidth(m.canvasWidth()), cardHeight()
}
