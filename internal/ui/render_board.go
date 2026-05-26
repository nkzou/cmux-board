package ui

import (
	"fmt"
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
}

// renderBoard renders the kanban board given the provided parameters.
// It handles scroll indicators, column rendering, and active/hover states.
func renderBoard(p renderBoardParams) string {
	if len(p.columns) == 0 {
		return lipgloss.NewStyle().
			Foreground(p.colors.muted).
			Render("No columns configured.")
	}

	// Compute visible columns based on available width.
	const minColumnWidth = 20
	maxVisible := p.width / (minColumnWidth + 2)
	if maxVisible < 1 {
		maxVisible = 1
	}

	startCol := p.scrollOffset
	endCol := min(startCol+maxVisible, len(p.columns))

	numVisible := endCol - startCol
	baseWidth := p.width / numVisible
	if baseWidth < minColumnWidth {
		baseWidth = minColumnWidth
	}
	remainder := p.width - baseWidth*numVisible

	var columns []string

	if startCol > 0 {
		indicator := lipgloss.NewStyle().
			Foreground(p.colors.muted).
			Background(p.colors.surface).
			Padding(0, 1).
			Render(fmt.Sprintf("◀ %d", startCol))
		columns = append(columns, indicator)
	}

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
			col:              col,
			tickets:          tickets,
			isActive:         isActive,
			isDragTarget:     isDragTarget,
			isHovered:        isHovered,
			width:            colWidth,
			isLast:           isLast,
			ticketOffset:     ticketOffset,
			activeTicket:     p.activeTicket,
			hoverTicket:      p.hoverTicket,
			colors:           p.colors,
		}))
	}

	if endCol < len(p.columns) {
		remaining := len(p.columns) - endCol
		indicator := lipgloss.NewStyle().
			Foreground(p.colors.muted).
			Background(p.colors.surface).
			Padding(0, 1).
			Render(fmt.Sprintf("%d ▶", remaining))
		columns = append(columns, indicator)
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
