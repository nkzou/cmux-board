package ui

import "github.com/charmbracelet/lipgloss"

// HelpText returns a plain string listing all active keyboard shortcuts.
// Used by tests and by renderHelp.
func HelpText() string {
	return "" +
		"  " + KeyImportJira + ":   import Jira ticket by key\n" +
		"  " + KeyCreateLocal + ":   create local ticket\n" +
		"  " + KeyRemoveTicket + ":   remove selected ticket from board\n" +
		"  " + KeyCycleStatus + ":   cycle status (Open -> In Progress -> Done)\n" +
		"  " + KeyZCycleNext + ":  cycle through tickets (forward)\n" +
		"  shift+" + KeyZCycleNext + ": cycle through tickets (backward)\n" +
		"  enter: activate selected ticket\n" +
		"  m:     manage activations for ticket\n" +
		"  N:     new approach (skip existing activation check)\n" +
		"  a:     assign repos to ticket\n" +
		"  /:     filter tickets\n" +
		"  ?:     toggle this help\n" +
		"  q:     quit"
}

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
		"  " + keyStyle.Render(KeyImportJira) + descStyle.Render("       import Jira ticket by key") + "\n" +
		"  " + keyStyle.Render(KeyCreateLocal) + descStyle.Render("       create local ticket") + "\n" +
		"  " + keyStyle.Render(KeyRemoveTicket) + descStyle.Render("       remove selected ticket from board") + "\n" +
		"  " + keyStyle.Render(KeyCycleStatus) + descStyle.Render("       cycle status (Open -> In Progress -> Done)") + "\n" +
		"  " + keyStyle.Render("Tab") + descStyle.Render("     cycle through tickets") + "\n" +
		"  " + keyStyle.Render("Enter") + descStyle.Render("   activate ticket (open worktree or focus)") + "\n" +
		"  " + keyStyle.Render("h/j/k/l") + descStyle.Render(" navigate to nearest post-it") + "\n" +
		"  " + keyStyle.Render("m") + descStyle.Render("       manage activations for ticket") + "\n" +
		"  " + keyStyle.Render("N") + descStyle.Render("       new approach (skips existing activation check)") + "\n" +
		"  " + keyStyle.Render("a") + descStyle.Render("       assign repos to ticket") + "\n" +
		"  " + keyStyle.Render("/") + descStyle.Render("       filter tickets") + "\n" +
		"  " + keyStyle.Render("?") + descStyle.Render("       toggle this help") + "\n" +
		"  " + keyStyle.Render("q") + descStyle.Render("       quit") + "\n\n" +
		sep + "\n" +
		sectionStyle.Render("  Picker  ") + descStyle.Render("(activation list)") + "\n" +
		sep + "\n" +
		"  " + keyStyle.Render("j/k") + descStyle.Render("     navigate activations") + "\n" +
		"  " + keyStyle.Render("Enter") + descStyle.Render("   focus selected activation") + "\n" +
		"  " + keyStyle.Render("n") + descStyle.Render("       new approach") + "\n" +
		"  " + keyStyle.Render("d") + descStyle.Render("       delete activation") + "\n" +
		"  " + keyStyle.Render("Esc") + descStyle.Render("     cancel") + "\n\n" +
		sep + "\n" +
		sectionStyle.Render("  Assign  ") + descStyle.Render("(repo assignment editor)") + "\n" +
		sep + "\n" +
		"  " + keyStyle.Render("j/k") + descStyle.Render("     navigate repos") + "\n" +
		"  " + keyStyle.Render("Space") + descStyle.Render("   toggle assignment") + "\n" +
		"  " + keyStyle.Render("Enter") + descStyle.Render("   confirm") + "\n" +
		"  " + keyStyle.Render("Esc") + descStyle.Render("     cancel") + "\n\n" +
		sep + "\n" +
		"  " + dimStyle.Render("Press any key to close")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colors.primary).
		Padding(1, 2).
		Render(help)
}
