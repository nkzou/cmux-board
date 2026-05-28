package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	"github.com/nkzou/cmux-board/internal/state"
)

// renderPostitCanvas renders all post-its at their (x, y) coordinates on a 2D canvas.
// It replaces the stub added in T-402a.
//
// Pipeline (per Review F-07):
//  1. Reconcile zOrder against snap.Tickets (local copy — View() is pure).
//  2. Allocate lines[]string for canvas height.
//  3. For each card bottom-up (ascending zOrder), render with renderTicket,
//     wrap with zone.Mark(key, cardStr) before composition, then splice into lines.
//  4. Return strings.Join(lines, "\n").
//
// zone.Scan is NOT called here; it is called once at root in view.go (T-601 lifecycle).
// Note: View() is called with a value receiver; zOrder reconciliation here updates a
// local copy. Persistent reconciliation happens in Update() (T-403 follow-up).
func (m *Model) renderPostitCanvas() string {
	snap, _ := m.store.Snapshot()
	m.reconcileZOrder(snap)

	h := m.canvasHeight()
	w := m.width
	if w <= 0 {
		w = defaultTerminalWidth
	}

	if len(snap.Tickets) == 0 && m.mode == ModeNormal {
		return centeredHint(w, h,
			"Press `i` to import a Jira ticket, `c` to create a local post-it. Press `?` for help.")
	}

	// Allocate one string per canvas row, each filled with spaces.
	lines := make([]string, h)
	for i := range lines {
		lines[i] = strings.Repeat(" ", w)
	}

	colors := defaultColors()

	// Render bottom-up: cards early in zOrder are under cards later in zOrder.
	for _, k := range m.zOrder {
		ticket, ok := snap.Tickets[k]
		if !ok {
			continue
		}

		// Render the card body.
		cardW := cardWidth(w)
		cardStr := renderTicket(renderTicketParams{
			ticket:       ticketStateToUI(ticket, m.activatingTickets[k], state.ActivationCount(snap, k)),
			isSelected:   k == m.selectedKey,
			filteredOut:  m.ticketFilteredOut(ticket),
			width:        cardW,
			accentColor:  colors.primary,
			colors:       colors,
			spinnerGlyph: m.spinnerGlyph(),
		})

		// Wrap with zone.Mark BEFORE splitting into lines (one rectangle per card).
		// Guard: zone.DefaultManager is nil until zone.NewGlobal() is called (T-601).
		var marked string
		if zone.DefaultManager != nil {
			marked = zone.Mark(k, cardStr)
		} else {
			marked = cardStr
		}

		// Split into card lines.
		cardLines := strings.Split(marked, "\n")

		// Use live drag position when this card is being dragged so motion is smooth.
		// The committed (x, y) only changes on release; without this, the card would
		// stay put until release and then snap to the new spot.
		drawX, drawY := ticket.X, ticket.Y
		if m.dragging != nil && m.dragging.key == k && m.dragging.motion {
			drawX, drawY = m.dragging.currentX, m.dragging.currentY
		}

		// Splice each card line into the canvas at (drawX, drawY + i).
		for i, cl := range cardLines {
			y := drawY + i
			if y < 0 || y >= len(lines) {
				continue
			}
			lines[y] = spliceLine(lines[y], drawX, cl, w)
		}
	}

	return strings.Join(lines, "\n")
}

const (
	defaultTerminalWidth  = 80
	defaultTerminalHeight = 24
	headerRows            = 2
	statusRows            = 1
)

func (m Model) canvasHeight() int {
	h := m.height
	if h <= 0 {
		h = defaultTerminalHeight
	}
	h -= headerRows + statusRows
	if h < 1 {
		return 1
	}
	return h
}

// reconcileZOrder synchronizes m.zOrder with snap.Tickets.
//
// Contract (Review F-06):
//   - Keys in snap.Tickets but not in m.zOrder are appended (new tickets go to the bottom).
//   - Keys in m.zOrder but not in snap.Tickets are pruned.
//   - Relative order of surviving keys is preserved.
//   - If m.selectedKey is no longer in snap.Tickets, it is reset to "".
//     (chosen behavior: empty over auto-select top — documented here.)
func (m *Model) reconcileZOrder(snap *state.State) {
	inSnap := make(map[string]bool, len(snap.Tickets))
	for k := range snap.Tickets {
		inSnap[k] = true
	}

	pruned := make([]string, 0, len(m.zOrder))
	inOrder := make(map[string]bool, len(m.zOrder))
	for _, k := range m.zOrder {
		if inSnap[k] {
			pruned = append(pruned, k)
			inOrder[k] = true
		}
	}
	// Append new keys (stable iteration order via sorted snapshot is ideal but not
	// required — arrival order is acceptable for personal-scale boards).
	for k := range snap.Tickets {
		if !inOrder[k] {
			pruned = append(pruned, k)
		}
	}
	m.zOrder = pruned

	if m.selectedKey != "" && !inSnap[m.selectedKey] {
		m.selectedKey = ""
	}
	// Auto-select the topmost ticket when nothing is selected and tickets exist,
	// so keyboard-driven actions (x, s, arrows, Enter) always have a target.
	// Without this, a fresh user with no clicks can't trigger any keybind that
	// requires a selection.
	if m.selectedKey == "" && len(m.zOrder) > 0 {
		m.selectedKey = m.zOrder[len(m.zOrder)-1]
	}
}

// spliceLine inserts ins into dst at column atX, clipping to total width.
// All slicing is cell-aware via ansi.Cut so a previously spliced card's
// SGR escapes don't break when a second card is placed on the same row
// (rune-counting was the bug: ANSI escape bytes shifted column math).
func spliceLine(dst string, atX int, ins string, total int) string {
	if atX < 0 {
		atX = 0
	}
	if atX >= total {
		return dst
	}

	insW := lipgloss.Width(ins)
	if insW <= 0 {
		return dst
	}

	endX := atX + insW
	if endX > total {
		ins = ansi.Truncate(ins, total-atX, "")
		endX = total
	}

	prefix := ansi.Cut(dst, 0, atX)
	suffix := ansi.Cut(dst, endX, total)

	return prefix + ins + suffix
}

// centeredHint returns a string of the given canvas dimensions with a centered
// help hint. Used for the first-run empty-board case (Review F-11).
func centeredHint(w, h int, hint string) string {
	if w <= 0 {
		w = defaultTerminalWidth
	}
	if h <= 0 {
		h = defaultTerminalHeight
	}

	style := lipgloss.NewStyle().
		Foreground(defaultColors().muted).
		Width(w).
		Align(lipgloss.Center)

	rendered := style.Render(hint)

	topPad := (h - 1) / 2
	lines := make([]string, h)
	for i := range lines {
		if i == topPad {
			lines[i] = rendered
		} else {
			lines[i] = strings.Repeat(" ", w)
		}
	}
	return strings.Join(lines, "\n")
}

// cardWidth returns a reasonable default card width based on canvas width.
// Cards are roughly 1/3 of the canvas width, clamped to sensible bounds.
func cardWidth(canvasW int) int {
	w := canvasW / 3
	if w < 16 {
		w = 16
	}
	if w > 40 {
		w = 40
	}
	return w
}
