package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderTicketParams holds all parameters for renderTicket.
type renderTicketParams struct {
	ticket      Ticket
	isSelected  bool
	isHovered   bool
	filteredOut bool
	width       int
	accentColor lipgloss.Color
	colors      uiColors
	// spinnerGlyph is the current frame to draw next to the "activating" label
	// when ticket.IsActivating is true. Empty when no activation is in flight.
	spinnerGlyph string
}

// statusBorder maps the ticket's effective status to a border color.
// For local tickets the LocalStatus field is used; for Jira tickets the Status field is used.
// F6 invariant: border color is purely visual and never affects position.
func statusBorder(t Ticket, colors uiColors) lipgloss.Color {
	s := t.Status
	if t.Source == "local" {
		s = t.LocalStatus
	}
	switch s {
	case "Open", "To Do":
		return colors.BorderOpen
	case "In Progress":
		return colors.BorderInProgress
	case "Done":
		return colors.BorderDone
	default:
		return colors.BorderNeutral
	}
}

// renderTicket renders a single kanban ticket card.
// TODO(M-006): add [orphan] glyph when claude_orphan:true
func renderTicket(p renderTicketParams) string {
	if p.filteredOut {
		p.colors = dimmedColors(p.colors)
		p.accentColor = p.colors.muted
	}

	var headerParts []string

	// Source badge for local tickets.
	if p.ticket.Source == "local" {
		localStyle := lipgloss.NewStyle().
			Foreground(p.colors.subtext).
			Background(p.colors.overlay).
			Padding(0, 1)
		headerParts = append(headerParts, localStyle.Render("(local)"))
	}

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
	// Local tickets show LocalStatus; Jira tickets show Status.
	displayStatus := p.ticket.Status
	if p.ticket.Source == "local" {
		displayStatus = p.ticket.LocalStatus
	}
	if displayStatus != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(p.colors.base).
			Background(p.colors.primary).
			Padding(0, 1)
		headerParts = append(headerParts, statusStyle.Render(displayStatus))
	}

	// Worktree badge — present when this ticket already has at least one
	// activation. Tells the user a press-Enter will switch to the existing
	// claude/worktree workspace instead of creating a new one. ⎇ is the
	// Unicode "alternative key" glyph commonly used for branch indicators.
	if p.ticket.ActivationCount > 0 {
		label := "⎇"
		if p.ticket.ActivationCount > 1 {
			label = fmt.Sprintf("⎇%d", p.ticket.ActivationCount)
		}
		worktreeStyle := lipgloss.NewStyle().
			Foreground(p.colors.success).
			Bold(true)
		headerParts = append(headerParts, worktreeStyle.Render(label))
	}

	// Activating badge — spinner glyph + label rendered on the right end of the
	// header. Signals that an Activate goroutine is in flight so the user
	// doesn't fire a second one.
	if p.ticket.IsActivating {
		glyph := p.spinnerGlyph
		if glyph == "" {
			glyph = "⠋"
		}
		activatingStyle := lipgloss.NewStyle().
			Foreground(p.colors.warning).
			Bold(true)
		headerParts = append(headerParts, activatingStyle.Render(glyph+" activating"))
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
	borderColor := statusBorder(p.ticket, p.colors)

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
		Width(p.width)
	if p.filteredOut {
		cardStyle = cardStyle.Faint(true)
	}
	// No MarginBottom — a trailing empty row inside zone.Mark would extend the
	// drag hit zone past the visible card, breaking the "what I see is what I
	// can click" expectation on a freeform 2D board.

	return cardStyle.Render(content)
}

func dimmedColors(colors uiColors) uiColors {
	colors.text = colors.muted
	colors.subtext = colors.muted
	colors.primary = colors.overlay
	colors.secondary = colors.muted
	colors.success = colors.muted
	colors.warning = colors.muted
	colors.err = colors.muted
	colors.info = colors.muted
	colors.BorderOpen = colors.muted
	colors.BorderInProgress = colors.muted
	colors.BorderDone = colors.muted
	colors.BorderNeutral = colors.muted
	return colors
}
