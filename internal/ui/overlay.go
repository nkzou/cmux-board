package ui

import "github.com/charmbracelet/lipgloss"

// renderWithOverlay places an overlay dialog centered on a background of the given dimensions.
func renderWithOverlay(width, height int, overlay string, colors uiColors) string {
	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(colors.base),
	)
}

// renderConfirmDialog renders a simple yes/no confirmation dialog.
// msg is the confirmation question to display.
func renderConfirmDialog(msg string, colors uiColors) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(colors.err).
		Bold(true)

	dimStyle := lipgloss.NewStyle().Foreground(colors.muted)

	content := titleStyle.Render("⚠ Confirm") + "\n\n" +
		"  " + lipgloss.NewStyle().Foreground(colors.text).Render(msg) + "\n\n" +
		"  " + lipgloss.NewStyle().Foreground(colors.success).Render("[y]") + dimStyle.Render(" Yes    ") +
		lipgloss.NewStyle().Foreground(colors.err).Render("[n]") + dimStyle.Render(" No    ") +
		lipgloss.NewStyle().Foreground(colors.muted).Render("[Esc]") + dimStyle.Render(" Cancel")

	return lipgloss.NewStyle().
		Border(columnBorder).
		BorderForeground(colors.err).
		Padding(1, 2).
		Render(content)
}
