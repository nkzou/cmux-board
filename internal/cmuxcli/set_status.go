package cmuxcli

import (
	"context"
	"fmt"
	"strconv"
)

// SetStatusOpts are optional overrides for SetStatus.
type SetStatusOpts struct {
	// WorkspaceRef overrides the default workspace target.
	// When empty, cmux uses $CMUX_WORKSPACE_ID (cmux-board's own workspace).
	WorkspaceRef string
	// Priority is the sort priority for the pill (higher = first). Default 0.
	Priority int
	// HasPriority, when true, adds --priority to the argv even when Priority == 0.
	HasPriority bool
}

// SetStatus calls `cmux set-status <key> <value>` with the icon and color for key.
//
// Exact argv:
//
//	cmux set-status <key> <value> --icon <icon> --color <color> [--workspace <ref>] [--priority <n>]
//
// Key must be one of PillKeyTracker, PillKeyCmux, or PillKeyClaude. Value is the
// human-readable status string (e.g. "online", "offline (since 14:30)", "unreachable").
// Color is chosen from pillStyles: HealthyColor when value == "online" or "" (healthy/absent),
// OfflineColor otherwise.
//
// RESEARCH §7 flag summary:
//
//	cmux set-status <key> <value>
//	  --icon <name>         Icon name (e.g. "sparkle", "hammer")
//	  --color <#hex>        Pill color
//	  --priority <n>        Sort priority (higher first; default 0)
//	  --workspace <id|ref>  (default: $CMUX_WORKSPACE_ID)
func SetStatus(ctx context.Context, key PillKey, value string, opts SetStatusOpts) error {
	style, ok := pillStyles[key]
	if !ok {
		return fmt.Errorf("SetStatus: unknown pill key %q", key)
	}

	color := style.OfflineColor
	if value == "online" || value == "" {
		color = style.HealthyColor
	}

	args := []string{
		"set-status", string(key), value,
		"--icon", style.Icon,
		"--color", color,
	}
	if opts.WorkspaceRef != "" {
		args = append(args, "--workspace", opts.WorkspaceRef)
	}
	if opts.HasPriority {
		args = append(args, "--priority", strconv.Itoa(opts.Priority))
	}

	_, _, err := runCmux(ctx, args...)
	if err != nil {
		return fmt.Errorf("cmux set-status (key=%s, value=%s) failed: %w", key, value, err)
	}
	return nil
}

// SetTrackerOnline sets the tracker pill to the healthy "online" state.
func SetTrackerOnline(ctx context.Context) error {
	return SetStatus(ctx, PillKeyTracker, "online", SetStatusOpts{})
}

// SetTrackerOffline sets the tracker pill to the offline state with a since-time string.
// sinceHHMM should be formatted as "HH:MM" (e.g. "14:30").
func SetTrackerOffline(ctx context.Context, sinceHHMM string) error {
	return SetStatus(ctx, PillKeyTracker, fmt.Sprintf("offline (since %s)", sinceHHMM), SetStatusOpts{})
}

// SetCmuxUnreachable sets the cmux pill to indicate the socket is unreachable.
func SetCmuxUnreachable(ctx context.Context) error {
	return SetStatus(ctx, PillKeyCmux, "unreachable", SetStatusOpts{})
}

// SetClaudeDegraded sets the claude pill to indicate the agent view is unavailable.
func SetClaudeDegraded(ctx context.Context) error {
	return SetStatus(ctx, PillKeyClaude, "degraded", SetStatusOpts{})
}

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

	colorHealthyGreen = "#22c55e"
	colorTrackerAmber = "#f59e0b"
	colorCmuxRed      = "#ef4444"
	colorClaudePurple = "#a855f7"
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
