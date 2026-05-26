package cmuxcli

// PillKey identifies one of the three cmux-board status pills.
type PillKey string

const (
	// PillKeyTracker is the Jira sync status pill.
	PillKeyTracker PillKey = "tracker"
	// PillKeyCmux is the cmux connectivity status pill.
	PillKeyCmux PillKey = "cmux"
	// PillKeyClaude is the Claude agent view status pill.
	PillKeyClaude PillKey = "claude"
)

// Named color and icon constants — no magic strings in the wrapper code.
const (
	iconCloud    = "cloud"
	iconTerminal = "terminal"
	iconSparkle  = "sparkle"

	colorHealthyGreen   = "#22c55e"
	colorTrackerAmber   = "#f59e0b"
	colorCmuxRed        = "#ef4444"
	colorClaudePurple   = "#a855f7"
)

// PillStyleData defines the icon name and healthy/offline colors for a status pill.
// Colors are CSS hex strings.
type PillStyleData struct {
	Icon         string
	HealthyColor string
	OfflineColor string
}

// pillStyles maps each PillKey to its display constants (RESEARCH §7 "cmux set-status").
var pillStyles = map[PillKey]PillStyleData{
	PillKeyTracker: {Icon: iconCloud, HealthyColor: colorHealthyGreen, OfflineColor: colorTrackerAmber},
	PillKeyCmux:    {Icon: iconTerminal, HealthyColor: colorHealthyGreen, OfflineColor: colorCmuxRed},
	PillKeyClaude:  {Icon: iconSparkle, HealthyColor: colorHealthyGreen, OfflineColor: colorClaudePurple},
}

// PillStyle returns the display constants for key.
// Returns false for the ok bool if key is unknown.
func PillStyle(key PillKey) (PillStyleData, bool) {
	s, ok := pillStyles[key]
	return s, ok
}
