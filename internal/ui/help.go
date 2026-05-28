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
		sectionStyle.Render("  Normal") + "\n" +
		sep + "\n" +
		"  " + keyStyle.Render("h/l") + descStyle.Render("     Move between columns") + "\n" +
		"  " + keyStyle.Render("j/k") + descStyle.Render("     Move between tickets") + "\n" +
		"  " + keyStyle.Render("Enter") + descStyle.Render("   Activate ticket (open worktree or focus)") + "\n" +
		"  " + keyStyle.Render("m") + descStyle.Render("       Manage activations for ticket") + "\n" +
		"  " + keyStyle.Render("N") + descStyle.Render("       New approach (skips existing activation check)") + "\n" +
		"  " + keyStyle.Render("a") + descStyle.Render("       Assign repos to ticket") + "\n" +
		"  " + keyStyle.Render("/") + descStyle.Render("       Filter tickets") + "\n" +
		"  " + keyStyle.Render("?") + descStyle.Render("       Toggle this help") + "\n" +
		"  " + keyStyle.Render("q") + descStyle.Render("       Quit") + "\n\n" +
		sep + "\n" +
		sectionStyle.Render("  Picker  ") + descStyle.Render("(activation list)") + "\n" +
		sep + "\n" +
		"  " + keyStyle.Render("j/k") + descStyle.Render("     Navigate activations") + "\n" +
		"  " + keyStyle.Render("Enter") + descStyle.Render("   Focus selected activation") + "\n" +
		"  " + keyStyle.Render("n") + descStyle.Render("       New approach") + "\n" +
		"  " + keyStyle.Render("d") + descStyle.Render("       Delete activation") + "\n" +
		"  " + keyStyle.Render("Esc") + descStyle.Render("     Cancel") + "\n\n" +
		sep + "\n" +
		sectionStyle.Render("  Assign  ") + descStyle.Render("(repo assignment editor)") + "\n" +
		sep + "\n" +
		"  " + keyStyle.Render("j/k") + descStyle.Render("     Navigate repos") + "\n" +
		"  " + keyStyle.Render("Space") + descStyle.Render("   Toggle assignment") + "\n" +
		"  " + keyStyle.Render("Enter") + descStyle.Render("   Confirm") + "\n" +
		"  " + keyStyle.Render("Esc") + descStyle.Render("     Cancel") + "\n\n" +
		sep + "\n" +
		"  " + dimStyle.Render("Press any key to close")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colors.primary).
		Padding(1, 2).
		Render(help)
}
