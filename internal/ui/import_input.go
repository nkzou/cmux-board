package ui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// handleImportInputMode handles key events in ModeImportInput.
// Enter commits the import; Esc cancels. All other keys are forwarded to the text input.
func (m Model) handleImportInputMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case KeyActivate: // "enter"
		key := strings.TrimSpace(m.importInput.Value())
		m.importInput.SetValue("")
		m.importInput.Blur()
		m.mode = ModeNormal
		if key == "" {
			return m, nil
		}
		return m, importJiraCmd(m.ctx, m.tr, m.store, key)
	case KeyEsc:
		m.importInput.SetValue("")
		m.importInput.Blur()
		m.mode = ModeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.importInput, cmd = m.importInput.Update(msg)
		return m, cmd
	}
}

// importJiraCmd returns a tea.Cmd that calls tr.GetTicket and on success
// writes the ticket into state via state.ImportJiraTicket.
// On any error, a PollErrMsg is returned as the tea.Msg.
func importJiraCmd(ctx context.Context, tr tracker.IssueTracker, store *state.Store, key string) tea.Cmd {
	return func() tea.Msg {
		if tr == nil {
			return PollErrMsg{Code: "no_tracker", When: time.Now()}
		}
		t, err := tr.GetTicket(ctx, key)
		if err != nil {
			return PollErrMsg{Code: "import_err", When: time.Now()}
		}
		if err := store.Mutate(func(s *state.State) error {
			state.ImportJiraTicket(s, t)
			return nil
		}); err != nil {
			return PollErrMsg{Code: "mutate_err", When: time.Now()}
		}
		return PollOKMsg{}
	}
}
