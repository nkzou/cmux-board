package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/nkzou/cmux-board/internal/state"
)

// View implements tea.Model. Composes all render helpers into the final TUI string.
// It is a pure function — no state mutations, no I/O.
func (m Model) View() string {
	colors := defaultColors()

	base := lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(colors),
		m.renderPostitCanvas(),
		m.renderModelStatusBar(colors),
	)

	// Apply mode-specific overlays.
	var result string
	switch m.mode {
	case ModePicker:
		result = m.renderPicker()
	case ModeApproachName:
		result = renderWithOverlay(m.width, m.height, m.renderApproachNamePrompt(colors), colors)
	case ModeAssignmentEditor:
		result = m.renderAssignmentEditor()
	case ModeRepoPicker:
		result = m.renderRepoPicker()
	case ModeHelp:
		result = renderWithOverlay(m.width, m.height, renderHelp(colors), colors)
	case ModeConfirm:
		result = renderWithOverlay(m.width, m.height, renderConfirmDialog("Are you sure?", colors), colors)
	default:
		// Normal, Filter, ShuttingDown: just the base layout.
		result = base
	}

	// Append toasts unconditionally on top when queue non-empty.
	if len(m.toasts) > 0 {
		result = lipgloss.JoinVertical(lipgloss.Left, result, m.renderToasts(colors))
	}

	return result
}

// renderHeader renders the top bar with board id/name on the left and status pills on the right.
func (m Model) renderHeader(colors uiColors) string {
	// T-402b will remove m.board; for now use cfg for label.
	var boardLabel string
	if m.cfg != nil {
		boardLabel = m.cfg.BoardID
	}

	left := lipgloss.NewStyle().
		Foreground(colors.primary).
		Bold(true).
		Render(boardLabel)

	right := m.renderStatusPills(colors)

	width := m.width
	if width < 10 {
		width = 80 // fallback for tests
	}
	spacing := width - lipgloss.Width(left) - lipgloss.Width(right)
	if spacing < 0 {
		spacing = 0
	}

	headerLine := lipgloss.JoinHorizontal(lipgloss.Center,
		left, strings.Repeat(" ", spacing), right)

	sep := lipgloss.NewStyle().
		Foreground(colors.muted).
		Render(strings.Repeat("─", width))

	return lipgloss.JoinVertical(lipgloss.Left, headerLine, sep)
}

// renderStatusPills renders the three status pills (tracker, cmux, claude) as inline badges.
func (m Model) renderStatusPills(colors uiColors) string {
	format := func(key string, pill pillState) string {
		text := pill.text
		if text == "" {
			text = key + ": ok"
		}
		var fg lipgloss.Color
		if strings.Contains(text, "offline") ||
			strings.Contains(text, "error") ||
			strings.Contains(text, "unreachable") ||
			strings.Contains(text, "re-auth") {
			fg = colors.err
		} else {
			fg = colors.muted
		}
		return lipgloss.NewStyle().
			Foreground(fg).
			Padding(0, 1).
			Render(text)
	}

	tracker := format(PillKeyTracker, m.trackerPill)
	cmuxPill := format(PillKeyCmux, m.cmuxPill)
	claude := format(PillKeyClaude, m.claudePill)

	sep := lipgloss.NewStyle().Foreground(colors.overlay).Render("│")
	return lipgloss.JoinHorizontal(lipgloss.Center, tracker, sep, cmuxPill, sep, claude)
}

// renderModelStatusBar renders the status bar using current mode and filter state.
func (m Model) renderModelStatusBar(colors uiColors) string {
	modeStr := m.modeString()

	// In filter mode, show the filter input value in the notification slot.
	var notif string
	if m.mode == ModeFilter {
		notif = fmt.Sprintf("Filter: %s", m.filterInput.View())
	}

	return renderStatusBar(renderStatusBarParams{
		mode:         modeStr,
		width:        m.width,
		notification: notif,
		colors:       colors,
	})
}

// renderApproachNamePrompt renders the approach name input as centered overlay content.
func (m Model) renderApproachNamePrompt(colors uiColors) string {
	titleStyle := lipgloss.NewStyle().Foreground(colors.primary).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colors.muted)

	content := titleStyle.Render("New approach name") + "\n" +
		dimStyle.Render("(Enter to confirm, Esc to cancel)") + "\n\n" +
		m.approachNameInput.View()

	return lipgloss.NewStyle().
		Border(columnBorder).
		BorderForeground(colors.primary).
		Padding(1, 3).
		Render(content)
}

// renderToasts renders the active toast queue as a stacked list at the bottom.
func (m Model) renderToasts(colors uiColors) string {
	if len(m.toasts) == 0 {
		return ""
	}
	toastStyle := lipgloss.NewStyle().
		Background(colors.surface).
		Foreground(colors.text).
		Padding(0, 1)

	lines := make([]string, 0, len(m.toasts))
	for _, t := range m.toasts {
		lines = append(lines, toastStyle.Render(t.msg))
	}
	return strings.Join(lines, "\n")
}

// modeString returns the uppercase display string for the current mode.
func (m Model) modeString() string {
	switch m.mode {
	case ModeNormal:
		return "NORMAL"
	case ModePicker, ModeRepoPicker:
		return "PICKER"
	case ModeApproachName:
		return "INSERT"
	case ModeAssignmentEditor:
		return "ASSIGN"
	case ModeFilter:
		return "FILTER"
	case ModeHelp:
		return "HELP"
	case ModeConfirm:
		return "CONFIRM"
	default:
		return "NORMAL"
	}
}

// ticketStateToUI converts a state.TicketState to the placeholder Ticket type.
// isActivating reflects whether an Activate goroutine is currently running for
// this ticket; activationCount is the number of existing worktrees. Both drive
// header badges in renderTicket.
func ticketStateToUI(t state.TicketState, isActivating bool, activationCount int) Ticket {
	return Ticket{
		Key:             t.Key,
		Summary:         t.Summary,
		Status:          t.Status,
		Labels:          t.Labels,
		Priority:        t.Priority,
		URL:             t.URL,
		Source:          t.Source,
		LocalStatus:     t.LocalStatus,
		IsActivating:    isActivating,
		ActivationCount: activationCount,
	}
}

// termenvAscii is referenced to ensure the termenv import is used in tests.
// Tests call lipgloss.SetColorProfile(termenv.Ascii) for deterministic output.
var _ termenv.Profile = termenv.Ascii
