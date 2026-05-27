package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// spinnerTickInterval is the cadence at which the activating-ticket spinner
// advances. 100ms matches typical TUI spinner speeds — fast enough to read as
// motion, slow enough not to dominate the event loop.
const spinnerTickInterval = 100 * time.Millisecond

// spinnerFrames is the braille-dot spinner used for activating tickets.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinnerGlyph returns the spinner frame for the current model tick.
func (m Model) spinnerGlyph() string {
	if len(spinnerFrames) == 0 {
		return ""
	}
	return spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
}

// tryActivate guards ActivateCmd against double-activation of the same ticket.
// If an activation for ticketID is already in flight, a toast is pushed and no
// Cmd is dispatched. Otherwise the ticket is marked as activating, the spinner
// tick is armed (if it wasn't already), and ActivateCmd runs.
func (m Model) tryActivate(ticketID, repoID, approach string) (Model, tea.Cmd) {
	if m.activatingTickets[ticketID] {
		return m.pushToast(ticketID + " is already being activated")
	}
	if m.activatingTickets == nil {
		m.activatingTickets = make(map[string]bool)
	}
	wasEmpty := len(m.activatingTickets) == 0
	m.activatingTickets[ticketID] = true

	cmd := ActivateCmd(m.ctx, m.store, m.cfg, ticketID, repoID, approach)
	if wasEmpty {
		return m, tea.Batch(cmd, spinnerTickCmd())
	}
	return m, cmd
}

// spinnerTickCmd returns a tick that schedules the next spinner frame.
func spinnerTickCmd() tea.Cmd {
	return tea.Tick(spinnerTickInterval, func(_ time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// handleSpinnerTick advances the spinner frame and re-arms the tick only while
// at least one activation is in flight. When the set drains, the tick stops on
// its own.
func (m Model) handleSpinnerTick(_ spinnerTickMsg) (Model, tea.Cmd) {
	if len(m.activatingTickets) == 0 {
		return m, nil
	}
	m.spinnerFrame++
	return m, spinnerTickCmd()
}

// clearActivating removes ticketID from the in-flight set. Safe to call when
// the key is absent (e.g. duplicate done messages).
func (m Model) clearActivating(ticketID string) Model {
	if m.activatingTickets == nil {
		return m
	}
	delete(m.activatingTickets, ticketID)
	if len(m.activatingTickets) == 0 {
		m.spinnerFrame = 0
	}
	return m
}
