package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderBoardParams holds all parameters for renderBoard to keep the signature manageable.
type renderBoardParams struct {
	columns      []Column
	width        int
	colors       uiColors
	scrollOffset int
	activeColumn int
	// dragState
	dragging         bool
	dragTargetColumn int
	dragSourceColumn int
	hoverColumn      int
	// per-column data
	columnTickets  [][]Ticket
	columnOffsets  []int
	activeTicket   int
	hoverTicket    int
	sidebarFocused bool
	spinnerGlyph   string
}

// renderBoard renders the kanban board given the provided parameters.
// It handles scroll indicators, column rendering, and active/hover states.
//
// All configured columns are sized to fit within p.width. Columns share the
// available width evenly and shrink as needed — the previous behaviour clipped
// columns past width/22 behind ◀ ▶ indicators, which hid most of the board on
// the narrow Dock sidebar.
func renderBoard(p renderBoardParams) string {
	if len(p.columns) == 0 {
		return lipgloss.NewStyle().
			Foreground(p.colors.muted).
			Render("No columns configured.")
	}

	startCol := 0
	endCol := len(p.columns)
	numVisible := endCol - startCol

	// Account for the 1-cell right margin between adjacent columns. The last
	// column has no margin, so total margin overhead is (numVisible-1).
	available := p.width - (numVisible - 1)
	if available < numVisible {
		available = numVisible // at least 1 cell per column
	}
	baseWidth := available / numVisible
	if baseWidth < 1 {
		baseWidth = 1
	}
	remainder := available - baseWidth*numVisible

	var columns []string

	for i := startCol; i < endCol; i++ {
		col := p.columns[i]
		isActive := i == p.activeColumn && !p.sidebarFocused
		isLast := i == endCol-1
		isDragTarget := p.dragging && i == p.dragTargetColumn && i != p.dragSourceColumn
		isHovered := i == p.hoverColumn && !p.dragging

		colWidth := baseWidth
		if i-startCol < remainder {
			colWidth++
		}

		var tickets []Ticket
		if i < len(p.columnTickets) {
			tickets = p.columnTickets[i]
		}
		ticketOffset := 0
		if i < len(p.columnOffsets) {
			ticketOffset = p.columnOffsets[i]
		}

		columns = append(columns, renderColumn(renderColumnParams{
			col:          col,
			tickets:      tickets,
			isActive:     isActive,
			isDragTarget: isDragTarget,
			isHovered:    isHovered,
			width:        colWidth,
			isLast:       isLast,
			ticketOffset: ticketOffset,
			activeTicket: p.activeTicket,
			hoverTicket:  p.hoverTicket,
			colors:       p.colors,
			spinnerGlyph: p.spinnerGlyph,
		}))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, columns...)
}

// columnColorFor returns the header color for a column based on its name.
// TODO(M-008): replace with tracker-aware mapping from column ID to status color.
func columnColorFor(colName string, colors uiColors) lipgloss.Color {
	switch strings.ToLower(colName) {
	case "backlog", "todo", "open":
		return colors.primary
	case "in progress", "in_progress", "doing":
		return colors.warning
	case "done", "closed", "resolved":
		return colors.success
	default:
		return colors.muted
	}
}
