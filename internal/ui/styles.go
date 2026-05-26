package ui

import "github.com/charmbracelet/lipgloss"

// uiColors holds the resolved lipgloss colors for the current theme.
// Populated from config.Theme in M-008; defaults are provided here for the seed-crystal stage.
type uiColors struct {
	base      lipgloss.Color
	surface   lipgloss.Color
	overlay   lipgloss.Color
	text      lipgloss.Color
	subtext   lipgloss.Color
	muted     lipgloss.Color
	primary   lipgloss.Color
	secondary lipgloss.Color
	success   lipgloss.Color
	warning   lipgloss.Color
	err       lipgloss.Color
	info      lipgloss.Color
}

// defaultColors returns a sensible catppuccin-mocha palette for use before config is wired.
// TODO(M-008): replace with newUIColors(theme config.Theme).
func defaultColors() uiColors {
	return uiColors{
		base:      lipgloss.Color("#1e1e2e"),
		surface:   lipgloss.Color("#313244"),
		overlay:   lipgloss.Color("#45475a"),
		text:      lipgloss.Color("#cdd6f4"),
		subtext:   lipgloss.Color("#bac2de"),
		muted:     lipgloss.Color("#585b70"),
		primary:   lipgloss.Color("#89b4fa"),
		secondary: lipgloss.Color("#cba6f7"),
		success:   lipgloss.Color("#a6e3a1"),
		warning:   lipgloss.Color("#f9e2af"),
		err:       lipgloss.Color("#f38ba8"),
		info:      lipgloss.Color("#89dceb"),
	}
}

// Border styles (package-level singletons).
var (
	columnBorder = lipgloss.Border{
		Top:         "━",
		Bottom:      "━",
		Left:        "┃",
		Right:       "┃",
		TopLeft:     "┏",
		TopRight:    "┓",
		BottomLeft:  "┗",
		BottomRight: "┛",
	}

	columnBorderActive = lipgloss.Border{
		Top:         "━",
		Bottom:      "━",
		Left:        "┃",
		Right:       "┃",
		TopLeft:     "┏",
		TopRight:    "┓",
		BottomLeft:  "┗",
		BottomRight: "┛",
	}

	dragTargetBorder = lipgloss.Border{
		Top:         "═",
		Bottom:      "═",
		Left:        "║",
		Right:       "║",
		TopLeft:     "╔",
		TopRight:    "╗",
		BottomLeft:  "╚",
		BottomRight: "╝",
	}

	ticketBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
	}

	ticketBorderSelected = lipgloss.Border{
		Top:         "═",
		Bottom:      "═",
		Left:        "║",
		Right:       "║",
		TopLeft:     "╔",
		TopRight:    "╗",
		BottomLeft:  "╚",
		BottomRight: "╝",
	}
)
