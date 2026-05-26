package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderColumnParams holds all parameters for renderColumn.
type renderColumnParams struct {
	col          Column
	tickets      []Ticket
	isActive     bool
	isDragTarget bool
	isHovered    bool
	width        int
	isLast       bool
	ticketOffset int
	activeTicket int
	hoverTicket  int
	colors       uiColors
}

// renderColumn renders a single kanban column with its tickets.
func renderColumn(p renderColumnParams) string {
	headerColor := columnColorFor(p.col.Name, p.colors)

	icon := "○"
	if p.isActive {
		icon = "▸ " + icon
	}

	headerText := fmt.Sprintf("%s %s", icon, p.col.Name)

	countStyle := lipgloss.NewStyle().Foreground(p.colors.muted)
	countText := fmt.Sprintf("(%d)", len(p.tickets))

	header := lipgloss.NewStyle().
		Foreground(headerColor).
		Bold(true).
		Render(headerText)

	count := countStyle.Render(" " + countText)
	headerLine := header + count

	// Compute visible ticket window.
	const maxVisibleTickets = 10
	endIdx := min(p.ticketOffset+maxVisibleTickets, len(p.tickets))

	hasMoreAbove := p.ticketOffset > 0
	hasMoreBelow := endIdx < len(p.tickets)

	indicatorStyle := lipgloss.NewStyle().
		Foreground(p.colors.muted).
		Width(p.width - 4).
		Align(lipgloss.Center)

	var ticketViews []string

	if hasMoreAbove {
		ticketViews = append(ticketViews, indicatorStyle.Render(fmt.Sprintf("▲ %d more", p.ticketOffset)))
	}

	for i := p.ticketOffset; i < endIdx; i++ {
		ticket := p.tickets[i]
		isSelected := p.isActive && i == p.activeTicket
		isTicketHovered := p.isHovered && i == p.hoverTicket
		ticketViews = append(ticketViews, renderTicket(renderTicketParams{
			ticket:      ticket,
			isSelected:  isSelected,
			isHovered:   isTicketHovered,
			width:       p.width - 4,
			accentColor: headerColor,
			colors:      p.colors,
		}))
	}

	if hasMoreBelow {
		remaining := len(p.tickets) - endIdx
		ticketViews = append(ticketViews, indicatorStyle.Render(fmt.Sprintf("▼ %d more", remaining)))
	}

	ticketsView := strings.Join(ticketViews, "\n")
	if len(p.tickets) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(p.colors.muted).
			Italic(true).
			Padding(2, 0).
			Width(p.width - 4).
			Align(lipgloss.Center)
		ticketsView = emptyStyle.Render("○\nDrag or Space to move here")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, headerLine, "", ticketsView)

	border := columnBorder
	borderColor := p.colors.surface
	if p.isDragTarget {
		border = dragTargetBorder
		borderColor = p.colors.success
	} else if p.isActive {
		border = columnBorderActive
		borderColor = headerColor
	} else if p.isHovered {
		borderColor = p.colors.overlay
	}

	style := lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Width(p.width).
		Padding(0, 1)

	if !p.isLast {
		style = style.MarginRight(1)
	}

	return style.Render(content)
}
