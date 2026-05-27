package ui

import (
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/cmuxcli"
	"github.com/nkzou/cmux-board/internal/git"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// toastTTL is how long each toast remains visible before expiry.
const toastTTL = 3 * time.Second

// pushToast appends a toast entry to the queue and arms the expiry tick if the
// queue was previously empty. Returns the updated Model and the tick tea.Cmd
// (or nil when the queue was already non-empty and the tick is already armed).
// BubbleTea convention: value receiver; caller must use the returned Model.
func (m Model) pushToast(msg string) (Model, tea.Cmd) {
	wasEmpty := len(m.toasts) == 0
	m.toasts = append(m.toasts, toastEntry{
		msg:       msg,
		expiresAt: time.Now().Add(toastTTL),
	})
	if wasEmpty {
		return m, tea.Tick(toastTTL, func(_ time.Time) tea.Msg {
			return toastExpireMsg{}
		})
	}
	return m, nil
}

// expireToastsImpl removes expired toasts and re-arms the tick for the next live entry.
// Called from Update via the expireToasts dispatch.
func expireToastsImpl(m Model) (Model, tea.Cmd) {
	now := time.Now()
	live := m.toasts[:0]
	for _, t := range m.toasts {
		if !t.expiresAt.Before(now) {
			live = append(live, t)
		}
	}
	m.toasts = live
	if len(m.toasts) == 0 {
		return m, nil
	}
	// Re-arm for the next earliest expiry.
	next := time.Until(m.toasts[0].expiresAt)
	if next < 0 {
		next = 0
	}
	return m, tea.Tick(next, func(_ time.Time) tea.Msg {
		return toastExpireMsg{}
	})
}

// userFriendlyError maps known error sentinels to short actionable strings.
// All output MUST NOT include stack traces, internal file paths, or secret values.
func userFriendlyError(err error) string {
	if err == nil {
		return ""
	}
	// Known sentinels — checked in order of specificity.
	if errors.Is(err, git.ErrWorktreePathExists) {
		return "worktree path already exists — check ~/.config/cmux-board/state.json"
	}
	if errors.Is(err, cmuxcli.ErrCmuxUnreachable) {
		return "cmux unreachable — is cmux running?"
	}
	if errors.Is(err, tracker.ErrConflict) {
		return "tracker conflict — ticket was moved externally; board will refresh"
	}
	// Forward-declared sentinels (defined in later milestones):
	//   ErrUniquifyExhausted  — T-062b
	//   ErrClaudeVersionTooOld — M-006 T-040 (claudecli)
	// Checked by string match until those packages define the sentinel.
	if isErrUniquifyExhausted(err) {
		return "could not generate a unique branch name — too many approaches for this ticket"
	}
	if isErrClaudeVersionTooOld(err) {
		return "Claude version too old — run `npm i -g @anthropic-ai/claude-code` to upgrade"
	}
	return "activation failed — see logs"
}

// isErrUniquifyExhausted checks for the ErrUniquifyExhausted sentinel by identity.
// Until T-062b is implemented this always returns false.
func isErrUniquifyExhausted(err error) bool {
	// TODO(T-062b): replace with errors.Is(err, activate.ErrUniquifyExhausted).
	_ = err
	return false
}

// isErrClaudeVersionTooOld checks for the ErrClaudeVersionTooOld sentinel by identity.
// Until M-006 T-040 is implemented this always returns false.
func isErrClaudeVersionTooOld(err error) bool {
	// TODO(M-006 T-040): replace with errors.Is(err, claudecli.ErrClaudeVersionTooOld).
	_ = err
	return false
}
