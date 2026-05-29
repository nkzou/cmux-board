package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/state"
)

// handleCreateInputMode handles key events in ModeCreateInput.
// Enter commits the new local ticket; Esc cancels. All other keys are forwarded to the input.
func (m Model) handleCreateInputMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case KeyActivate: // "enter"
		name := strings.TrimSpace(m.createInput.Value())
		m.createInput.SetValue("")
		m.createInput.Blur()
		m.mode = ModeNormal
		if name == "" {
			return m, nil
		}
		if err := m.store.Mutate(func(s *state.State) error {
			_ = state.CreateLocalTicket(s, name)
			return nil
		}); err != nil {
			return m.pushToast("create failed: " + err.Error())
		}
		snap, rev := m.store.Snapshot()
		m.snapshot = snap
		m.snapshotRev = rev
		return m, nil
	case KeyEsc:
		m.createInput.SetValue("")
		m.createInput.Blur()
		m.mode = ModeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.createInput, cmd = m.createInput.Update(msg)
		return m, cmd
	}
}
