package ui

import (
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/cmuxcli"
)

// Pill key constants. Values MUST match cmuxcli.PillKey* exactly.
const (
	PillKeyTracker = string(cmuxcli.PillKeyTracker) // "tracker"
	PillKeyCmux    = string(cmuxcli.PillKeyCmux)    // "cmux"
	PillKeyClaude  = string(cmuxcli.PillKeyClaude)  // "claude"
)

// Pill text constants for the tracker pill (E4, E5).
const (
	PillTrackerOK      = "tracker: ok"
	PillTrackerOffline = "tracker: offline" // caller appends " (since HH:MM)" dynamically
	PillTrackerReauth  = "tracker: re-auth required — run cmux-board init --force"
)

// pillDebounceWindow suppresses duplicate emits for the same key+value within this window.
const pillDebounceWindow = time.Second

// noopPillMsg is a no-op result message emitted by the pill Cmd goroutine on completion.
type noopPillMsg struct{}

// emitPill emits a cmux set-status call for the given key and text, subject to debounce.
// Returns (updated Model, nil) if the same key+text was emitted within pillDebounceWindow.
// Errors from SetStatus are logged via slog but are not surfaced as toasts (best-effort).
// BubbleTea convention: value receiver; caller must use the returned Model.
func (m Model) emitPill(key, text string) (Model, tea.Cmd) {
	pill, ok := m.pillStateForKey(key)
	if !ok {
		return m, nil
	}
	if pill.text == text && time.Since(pill.lastEmitted) < pillDebounceWindow {
		return m, nil // suppress duplicate
	}
	pill.text = text
	pill.lastEmitted = time.Now()
	m.setPillState(key, pill)

	ctx := m.ctx
	pillKey := cmuxcli.PillKey(key)
	textCopy := text
	return m, func() tea.Msg {
		if err := cmuxcli.SetStatus(ctx, pillKey, textCopy, cmuxcli.SetStatusOpts{}); err != nil {
			slog.Error("pill SetStatus failed", "key", key, "text", textCopy, "err", err)
		}
		return noopPillMsg{}
	}
}

// pillStateForKey returns a copy of the pillState for the given key.
func (m Model) pillStateForKey(key string) (pillState, bool) {
	switch key {
	case PillKeyTracker:
		return m.trackerPill, true
	case PillKeyCmux:
		return m.cmuxPill, true
	case PillKeyClaude:
		return m.claudePill, true
	default:
		return pillState{}, false
	}
}

// setPillState stores the updated pillState for the given key.
func (m *Model) setPillState(key string, ps pillState) {
	switch key {
	case PillKeyTracker:
		m.trackerPill = ps
	case PillKeyCmux:
		m.cmuxPill = ps
	case PillKeyClaude:
		m.claudePill = ps
	}
}

// trackerOfflineText builds the display text for a tracker-offline pill.
// Format: "tracker: offline (since HH:MM)"
func trackerOfflineText(since time.Time) string {
	return fmt.Sprintf("%s (since %s)", PillTrackerOffline, since.Format("15:04"))
}
