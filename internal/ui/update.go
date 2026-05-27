package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model. Dispatches incoming messages to the appropriate handler.
// All handlers return (Model, tea.Cmd); Update is non-blocking.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	case PollOKMsg:
		return m.handlePollOK(msg)
	case PollErrMsg:
		return m.handlePollErr(msg)
	case PushOKMsg:
		return m.handlePushOK(msg)
	case PushConflictMsg:
		return m.handlePushConflict(msg)
	case snapshotRefreshMsg:
		return m.refreshSnapshot()
	case activationDoneMsg:
		return m.handleActivationDone(msg)
	case activationStepMsg:
		return m.handleActivationStep(msg)
	case toastExpireMsg:
		return m.expireToasts()
	default:
		return m, nil
	}
}

// handleWindowSize stores the new terminal dimensions.
func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	return m, nil
}

// handleKey routes key events to the per-mode handler.
// Per-mode handlers are implemented in keymap.go (T-057).
func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.mode {
	case ModeNormal:
		return m.handleNormalMode(msg)
	case ModePicker:
		return m.handlePickerMode(msg)
	case ModeApproachName:
		return m.handleApproachNameMode(msg)
	case ModeAssignmentEditor:
		return m.handleAssignmentEditorMode(msg)
	case ModeFilter:
		return m.handleFilterMode(msg)
	case ModeHelp:
		return m.handleHelpMode(msg)
	default:
		return m, nil
	}
}

// refreshSnapshot re-derives the board from the store.
// Called after PollOKMsg, snapshotRefreshMsg, and store mutations that need a UI refresh.
func (m Model) refreshSnapshot() (Model, tea.Cmd) {
	snap, rev := m.store.Snapshot()
	m.snapshot = snap
	m.snapshotRev = rev
	m.board = snap.Board

	mapped, unmapped := resolveTickets(snap)
	m.tickets = flattenMapped(m.board, mapped)
	m.unmappedTickets = unmapped

	return m, nil
}

// handlePollOK clears the poll-error state and refreshes the snapshot.
func (m Model) handlePollOK(msg PollOKMsg) (Model, tea.Cmd) {
	m.pollFailedAt = nil
	m.pollErrCode = ""
	m, cmd := m.refreshSnapshot()
	// TODO(T-058): emit PillTrackerOK via emitPill.
	_ = msg
	return m, cmd
}

// handlePollErr records the poll failure time and error code.
func (m Model) handlePollErr(msg PollErrMsg) (Model, tea.Cmd) {
	t := msg.When
	m.pollFailedAt = &t
	m.pollErrCode = msg.Code
	// TODO(T-058): emit pill update via emitPill.
	return m, nil
}

// handlePushOK is a stub; full implementation in T-065.
func (m Model) handlePushOK(_ PushOKMsg) (Model, tea.Cmd) {
	return m, nil
}

// handlePushConflict snaps the ticket back and queues a toast.
func (m Model) handlePushConflict(msg PushConflictMsg) (Model, tea.Cmd) {
	// TODO(T-065): call store.Mutate to snap ticket back to msg.ServerStatus.
	// TODO(T-058): emit cmux pill update.
	cmd := m.pushToast("conflict: " + msg.TicketID + " moved to " + msg.ServerStatus + " — snapped back")
	return m, cmd
}

// handleActivationDone handles activation completion or error.
func (m Model) handleActivationDone(msg activationDoneMsg) (Model, tea.Cmd) {
	if msg.Err != nil {
		cmd := m.pushToast(userFriendlyError(msg.Err))
		// TODO(T-058): emit cmux/claude pill update for specific error types.
		return m, cmd
	}
	return m, nil
}

// handleActivationStep is a stub; full implementation in T-062.
func (m Model) handleActivationStep(_ activationStepMsg) (Model, tea.Cmd) {
	return m, nil
}

// expireToasts removes stale toasts and re-arms the tick if any remain.
// Implemented in toast.go (T-059).
func (m Model) expireToasts() (Model, tea.Cmd) {
	return expireToastsImpl(m)
}
