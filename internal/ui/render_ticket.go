package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderTicketParams holds all parameters for renderTicket.
type renderTicketParams struct {
	ticket      Ticket
	isSelected  bool
	isHovered   bool
	width       int
	accentColor lipgloss.Color
	colors      uiColors
}

// renderTicket renders a single kanban ticket card.
// TODO(M-006): add [orphan] glyph when claude_orphan:true
func renderTicket(p renderTicketParams) string {
	var headerParts []string

	// Priority badge.
	if p.ticket.Priority == "highest" || p.ticket.Priority == "high" {
		pColor := p.colors.err
		label := "!"
		if p.ticket.Priority == "highest" {
			label = "!!"
			pColor = p.colors.err
		}
		headerParts = append(headerParts,
			lipgloss.NewStyle().Foreground(pColor).Bold(true).Render(label))
	}

	// Status badge (simple text for now; M-008 wires full status pill).
	if p.ticket.Status != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(p.colors.base).
			Background(p.colors.primary).
			Padding(0, 1)
		headerParts = append(headerParts, statusStyle.Render(p.ticket.Status))
	}

	headerLine := strings.Join(headerParts, "  ")

	titleStyle := lipgloss.NewStyle().
		Foreground(p.colors.text).
		Bold(p.isSelected).
		Width(p.width)
	wrappedTitle := titleStyle.Render(p.ticket.Summary)

	// Labels row.
	var labelParts []string
	for _, label := range p.ticket.Labels {
		lbl := lipgloss.NewStyle().
			Foreground(p.colors.subtext).
			Background(p.colors.overlay).
			Padding(0, 1).
			Render(label)
		labelParts = append(labelParts, lbl)
	}
	labelsLine := strings.Join(labelParts, " ")

	var lines []string
	if headerLine != "" {
		lines = append(lines, headerLine)
	}
	lines = append(lines, wrappedTitle)
	if labelsLine != "" {
		lines = append(lines, labelsLine)
	}

	content := strings.Join(lines, "\n")

	border := ticketBorder
	borderColor := p.colors.surface

	if p.isHovered && !p.isSelected {
		borderColor = p.colors.overlay
	}

	if p.isSelected {
		border = ticketBorderSelected
		borderColor = p.accentColor
	}

	cardStyle := lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(0, 1).
		MarginBottom(1).
		Width(p.width)

	return cardStyle.Render(content)
}
