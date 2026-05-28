package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderStatusBarParams holds parameters for renderStatusBar.
type renderStatusBarParams struct {
	mode         string
	width        int
	notification string
	colors       uiColors
}

// renderStatusBar renders the bottom status bar showing current mode, contextual hints,
// and any active notification.
func renderStatusBar(p renderStatusBarParams) string {
	type modeConfig struct {
		icon string
		bg   lipgloss.Color
	}

	modeConfigs := map[string]modeConfig{
		"NORMAL":  {"◆", p.colors.primary},
		"INSERT":  {"✎", p.colors.success},
		"PICKER":  {"◈", p.colors.info},
		"ASSIGN":  {"☑", p.colors.warning},
		"HELP":    {"?", p.colors.primary},
		"CONFIRM": {"!", p.colors.err},
		"FILTER":  {"/", p.colors.info},
	}

	cfg, ok := modeConfigs[p.mode]
	if !ok {
		cfg = modeConfig{"◆", p.colors.primary}
	}

	modeStr := lipgloss.NewStyle().
		Foreground(p.colors.base).
		Background(cfg.bg).
		Bold(true).
		Padding(0, 1).
		Render(cfg.icon + " " + p.mode)

	sep := lipgloss.NewStyle().Foreground(p.colors.overlay).Render(" │ ")
	hintStyle := lipgloss.NewStyle().Foreground(p.colors.subtext)
	dimStyle := lipgloss.NewStyle().Foreground(p.colors.muted)

	hints := contextualHints(p.mode, hintStyle, dimStyle, sep)

	notif := ""
	if p.notification != "" {
		isError := strings.HasPrefix(p.notification, "Failed") ||
			strings.HasPrefix(p.notification, "Error") ||
			strings.Contains(p.notification, "failed")
		bgColor := p.colors.success
		icon := "✓"
		if isError {
			bgColor = p.colors.err
			icon = "✗"
		}
		notif = lipgloss.NewStyle().
			Foreground(p.colors.base).
			Background(bgColor).
			Padding(0, 1).
			Render(icon + " " + p.notification)
	}

	left := lipgloss.JoinHorizontal(lipgloss.Center, modeStr, sep, hints)
	spacing := p.width - lipgloss.Width(left) - lipgloss.Width(notif)
	if spacing < 0 {
		spacing = 0
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, left, strings.Repeat(" ", spacing), notif)
}

// contextualHints returns keyboard hint text for the given mode.
func contextualHints(mode string, hintStyle, dimStyle lipgloss.Style, sep string) string {
	switch mode {
	case "NORMAL":
		return hintStyle.Render("h/l") + dimStyle.Render(" col") + sep +
			hintStyle.Render("j/k") + dimStyle.Render(" ticket") + sep +
			hintStyle.Render("Enter") + dimStyle.Render(" activate") + sep +
			hintStyle.Render("m") + dimStyle.Render(" manage") + sep +
			hintStyle.Render("N") + dimStyle.Render(" new approach") + sep +
			hintStyle.Render("a") + dimStyle.Render(" assign") + sep +
			hintStyle.Render("/") + dimStyle.Render(" filter") + sep +
			hintStyle.Render("?") + dimStyle.Render(" help") + sep +
			hintStyle.Render("q") + dimStyle.Render(" quit")
	case "PICKER":
		return hintStyle.Render("j/k") + dimStyle.Render(" navigate") + sep +
			hintStyle.Render("Enter") + dimStyle.Render(" focus") + sep +
			hintStyle.Render("n") + dimStyle.Render(" new") + sep +
			hintStyle.Render("d") + dimStyle.Render(" delete") + sep +
			hintStyle.Render("Esc") + dimStyle.Render(" cancel")
	case "ASSIGN":
		return hintStyle.Render("j/k") + dimStyle.Render(" navigate") + sep +
			hintStyle.Render("Space") + dimStyle.Render(" toggle") + sep +
			hintStyle.Render("Enter") + dimStyle.Render(" confirm") + sep +
			hintStyle.Render("Esc") + dimStyle.Render(" cancel")
	case "INSERT":
		return hintStyle.Render("Enter") + dimStyle.Render(" confirm") + sep +
			hintStyle.Render("Esc") + dimStyle.Render(" cancel")
	case "FILTER":
		return hintStyle.Render("Enter") + dimStyle.Render(" apply") + sep +
			hintStyle.Render("Esc") + dimStyle.Render(" clear")
	case "HELP":
		return dimStyle.Render("any key") + dimStyle.Render(" close")
	default:
		return hintStyle.Render("Esc") + dimStyle.Render(" back")
	}
}
