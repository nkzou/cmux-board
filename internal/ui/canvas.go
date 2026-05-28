package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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

	if len(snap.Tickets) == 0 && m.mode == ModeNormal {
		return centeredHint(m.width, m.height,
			"Press `i` to import a Jira ticket, `c` to create a local post-it. Press `?` for help.")
	}

	h := m.height
	if h <= 0 {
		h = 24
	}
	w := m.width
	if w <= 0 {
		w = 80
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

		// Splice each card line into the canvas at (ticket.X, ticket.Y + i).
		for i, cl := range cardLines {
			y := ticket.Y + i
			if y < 0 || y >= len(lines) {
				continue
			}
			lines[y] = spliceLine(lines[y], ticket.X, cl, w)
		}
	}

	return strings.Join(lines, "\n")
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
		m.selectedKey = "" // chosen behavior: empty over auto-select top
	}
}

// spliceLine inserts ins into dst at column atX, clipping to total width.
// Columns are measured using lipgloss.Width to handle ANSI-escaped strings.
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

	// Build prefix: first atX cells of dst.
	// dst may contain ANSI escapes; use rune-level trimming for simplicity.
	// (Post-its are positioned at terminal-cell units; plain spaces fill background.)
	prefix := runeSlice(dst, 0, atX)

	// Suffix: cells after the card's right edge, clamped to total.
	endX := atX + insW
	if endX > total {
		endX = total
	}
	suffix := runeSlice(dst, endX, total)

	return prefix + ins + suffix
}

// runeSlice returns the substring of s covering rune positions [start, end).
// Unlike a plain byte slice, this handles multi-byte characters correctly.
// ANSI escape sequences are treated as zero-width (best-effort; for plain
// space-filled backgrounds this is accurate).
func runeSlice(s string, start, end int) string {
	runes := []rune(s)
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	if start >= end {
		return ""
	}
	return string(runes[start:end])
}

// centeredHint returns a string of the given canvas dimensions with a centered
// help hint. Used for the first-run empty-board case (Review F-11).
func centeredHint(w, h int, hint string) string {
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
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
