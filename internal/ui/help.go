package ui

import "github.com/charmbracelet/lipgloss"

// renderHelp renders the keyboard shortcut help overlay.
func renderHelp(colors uiColors) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(colors.primary).
		Bold(true)

	sectionStyle := lipgloss.NewStyle().
		Foreground(colors.secondary).
		Bold(true)

	keyStyle := lipgloss.NewStyle().
		Foreground(colors.info).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(colors.subtext)

	sepStyle := lipgloss.NewStyle().
		Foreground(colors.surface)

	dimStyle := lipgloss.NewStyle().Foreground(colors.muted)

	sep := sepStyle.Render("────────────────────────────────────────────")

	help := titleStyle.Render("◈ Keyboard Shortcuts") + "\n\n" +
		sep + "\n" +
		sectionStyle.Render("  🧭 Navigation") + "                 " + sectionStyle.Render("📝 Actions") + "\n" +
		sep + "\n" +
		"  " + keyStyle.Render("h/l") + descStyle.Render("   Move between columns  ") + keyStyle.Render("n") + descStyle.Render("       New ticket") + "\n" +
		"  " + keyStyle.Render("j/k") + descStyle.Render("   Move between tickets  ") + keyStyle.Render("e") + descStyle.Render("       Edit ticket") + "\n" +
		"  " + keyStyle.Render("g") + descStyle.Render("     Go to first ticket    ") + keyStyle.Render("d") + descStyle.Render("       Delete ticket") + "\n" +
		"  " + keyStyle.Render("G") + descStyle.Render("     Go to last ticket     ") + keyStyle.Render("Space") + descStyle.Render("   Move forward") + "\n\n" +
		sep + "\n" +
		sectionStyle.Render("  👁 View") + "\n" +
		sep + "\n" +
		"  " + keyStyle.Render("/") + descStyle.Render("     Search/filter         ") + keyStyle.Render("O") + descStyle.Render("       Settings") + "\n" +
		"  " + keyStyle.Render("?") + descStyle.Render("     Toggle help           ") + keyStyle.Render("q") + descStyle.Render("       Quit") + "\n\n" +
		sep + "\n" +
		"  " + dimStyle.Render("Press any key to close")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colors.primary).
		Padding(1, 2).
		Render(help)
}
