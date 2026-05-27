package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/state"
)

// Key binding constants. All key string literals in handlers MUST use these constants;
// bare string literals in switch cases are forbidden (T-057 mandatory invariant 3).
const (
	// Normal mode navigation (vim-style)
	KeyLeft  = "h"
	KeyDown  = "j"
	KeyUp    = "k"
	KeyRight = "l"

	// Normal mode actions
	KeyActivate    = "enter"
	KeyNewApproach = "N" // capital N — new approach regardless of existing activations
	KeyAssignRepos = "a" // open assignment editor
	KeyFilter      = "/"
	KeyHelp        = "?"
	KeyQuit        = "q"

	// Picker mode
	KeyPickerFocus   = "enter"
	KeyPickerNew     = "n" // lowercase n — new approach from picker
	KeyPickerRespawn = "r"
	KeyPickerDelete  = "d"
	KeyPickerCancel  = "esc"

	// Assignment editor
	KeyAssignToggle = " " // space
	KeyAssignCommit = "enter"
	KeyAssignCancel = "esc"

	// Approach-name input
	KeyApproachCommit = "enter"
	KeyApproachCancel = "esc"

	// Universal
	KeyEsc = "esc"
)

// handleNormalMode handles key events when mode == ModeNormal.
func (m Model) handleNormalMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case KeyLeft, "left":
		if m.activeColIdx > 0 {
			m.activeColIdx--
			m.activeTicketIdx = 0
		}
	case KeyRight, "right":
		colCount := len(m.board.Columns)
		if m.activeColIdx < colCount-1 {
			m.activeColIdx++
			m.activeTicketIdx = 0
		}
	case KeyDown, "down":
		colTickets := m.ticketsForActiveCol()
		if m.activeTicketIdx < len(colTickets)-1 {
			m.activeTicketIdx++
		}
	case KeyUp, "up":
		if m.activeTicketIdx > 0 {
			m.activeTicketIdx--
		}
	case KeyActivate:
		// Determine current ticket.
		colTickets := m.ticketsForActiveCol()
		if len(colTickets) == 0 || m.activeTicketIdx >= len(colTickets) {
			return m, nil
		}
		ticketID := colTickets[m.activeTicketIdx].Key
		var res RepoResolution
		m, res = resolveRepoAndRoute(m, ticketID)
		if res.Cancelled || res.PickerOpened {
			// Cancelled → toast was pushed; PickerOpened → ModeRepoPicker is now active.
			return m, nil
		}
		// Exactly 1 repo assigned: check for existing activations.
		repoID := res.RepoID
		ps := newPickerState(m.snapshot, ticketID, repoID)
		if ps == nil {
			activations := state.FindActivations(m.snapshot, ticketID, repoID)
			if len(activations) == 0 {
				// 0 activations → activate with empty approach name.
				return m, ActivateCmd(m.ctx, m.store, m.cfg, ticketID, repoID, "")
			}
			// 1 activation → focus directly (T-063).
			return m.focusActivation(activations[0])
		}
		// 2+ activations → open activation picker.
		m.pickerState = ps
		m.mode = ModePicker
		return m, nil
	case KeyNewApproach:
		// Capital N: new approach regardless of existing activations (F16).
		// MUST NOT check len(activations) before entering ModeApproachName.
		m.previousMode = ModeNormal
		m.mode = ModeApproachName
		m.approachNameInput.SetValue("")
		m.approachNameInput.Focus()
	case KeyAssignRepos:
		colTickets := m.ticketsForActiveCol()
		if len(colTickets) == 0 || m.activeTicketIdx >= len(colTickets) {
			return m, nil
		}
		ticketID := colTickets[m.activeTicketIdx].Key
		m.assignmentEditor = newAssignmentEditorState(m.cfg, m.snapshot, ticketID)
		m.mode = ModeAssignmentEditor
	case KeyFilter:
		m.mode = ModeFilter
		m.filterInput.Focus()
	case KeyHelp:
		m.mode = ModeHelp
	case KeyQuit:
		m.mode = ModeShuttingDown
		return m, tea.Quit
	}
	return m, nil
}

// handlePickerMode handles key events when mode == ModePicker.
func (m Model) handlePickerMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.pickerState == nil {
		m.mode = ModeNormal
		return m, nil
	}
	ps := m.pickerState
	switch msg.String() {
	case KeyPickerFocus:
		if len(ps.entries) == 0 {
			return m, nil
		}
		entry := ps.entries[ps.cursorIdx]
		m.pickerState = nil
		m.mode = ModeNormal
		return m.focusActivation(entry)
	case KeyPickerNew:
		m.pickerState = nil
		m.previousMode = ModePicker
		m.mode = ModeApproachName
		m.approachNameInput.SetValue("")
		m.approachNameInput.Focus()
	case KeyPickerRespawn:
		// TODO(T-063): respawn selected activation.
		return m, nil
	case KeyPickerDelete:
		if len(ps.entries) == 0 {
			return m, nil
		}
		entry := ps.entries[ps.cursorIdx]
		activationID := entry.ActivationID
		if err := m.store.Mutate(func(s *state.State) error {
			return removeActivation(s, activationID)
		}); err != nil {
			return m.pushToast("delete failed: " + err.Error())
		}
		// Re-fetch from updated snapshot.
		snap, rev := m.store.Snapshot()
		m.snapshot = snap
		m.snapshotRev = rev
		newEntries := state.FindActivations(snap, ps.ticketID, ps.repoID)
		if len(newEntries) == 0 {
			m.pickerState = nil
			m.mode = ModeNormal
			return m, nil
		}
		m.pickerState = &pickerState{
			ticketID:  ps.ticketID,
			repoID:    ps.repoID,
			entries:   newEntries,
			cursorIdx: min(ps.cursorIdx, len(newEntries)-1),
		}
	case KeyPickerCancel: // == KeyEsc == "esc"
		m.pickerState = nil
		m.mode = ModeNormal
	case KeyDown, "down":
		if ps.cursorIdx < len(ps.entries)-1 {
			m.pickerState.cursorIdx++
		}
	case KeyUp, "up":
		if ps.cursorIdx > 0 {
			m.pickerState.cursorIdx--
		}
	}
	return m, nil
}

// handleApproachNameMode routes key events to the text input and handles commit/cancel.
func (m Model) handleApproachNameMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case KeyApproachCommit:
		name := m.approachNameInput.Value()
		if name == "" {
			return m, nil
		}
		m.approachNameInput.SetValue("")
		m.approachNameInput.Blur()
		m.mode = ModeNormal
		// Determine (ticketID, repoID) from context we came from.
		var ticketID, repoID string
		if m.previousMode == ModePicker && m.pickerState != nil {
			ticketID = m.pickerState.ticketID
			repoID = m.pickerState.repoID
			m.pickerState = nil
		} else {
			colTickets := m.ticketsForActiveCol()
			if len(colTickets) == 0 || m.activeTicketIdx >= len(colTickets) {
				return m, nil
			}
			ticketID = colTickets[m.activeTicketIdx].Key
			var res RepoResolution
			m, res = resolveRepoAndRoute(m, ticketID)
			if res.Cancelled || res.PickerOpened {
				return m, nil
			}
			repoID = res.RepoID
		}
		m.previousMode = ModeNormal
		return m, ActivateCmd(m.ctx, m.store, m.cfg, ticketID, repoID, name)
	case KeyApproachCancel:
		m.approachNameInput.SetValue("")
		m.approachNameInput.Blur()
		// Restore the mode we came from (ModeNormal or ModePicker).
		m.mode = m.previousMode
		m.previousMode = ModeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.approachNameInput, cmd = m.approachNameInput.Update(msg)
		return m, cmd
	}
}

// handleAssignmentEditorMode handles key events in ModeAssignmentEditor.
func (m Model) handleAssignmentEditorMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.assignmentEditor == nil {
		m.mode = ModeNormal
		return m, nil
	}
	ed := m.assignmentEditor
	switch msg.String() {
	case KeyAssignToggle:
		if len(ed.repoIDs) == 0 {
			return m, nil
		}
		id := ed.repoIDs[ed.cursorIdx]
		m.assignmentEditor.checked[id] = !m.assignmentEditor.checked[id]
	case KeyAssignCommit:
		// Build newAssigned: all repoIDs where checked == true, in repoIDs order.
		newAssigned := make([]string, 0, len(ed.repoIDs))
		for _, id := range ed.repoIDs {
			if ed.checked[id] {
				newAssigned = append(newAssigned, id)
			}
		}
		ticketID := ed.ticketID
		if err := m.store.Mutate(func(s *state.State) error {
			return state.ApplyAssignments(s, ticketID, newAssigned)
		}); err != nil {
			return m.pushToast("assignment failed: " + err.Error())
		}
		snap, rev := m.store.Snapshot()
		m.snapshot = snap
		m.snapshotRev = rev
		m.assignmentEditor = nil
		m.mode = ModeNormal
	case KeyAssignCancel:
		m.assignmentEditor = nil
		m.mode = ModeNormal
	case KeyDown, "down":
		if ed.cursorIdx < len(ed.repoIDs)-1 {
			m.assignmentEditor.cursorIdx++
		}
	case KeyUp, "up":
		if ed.cursorIdx > 0 {
			m.assignmentEditor.cursorIdx--
		}
	}
	return m, nil
}

// handleRepoPickerMode handles key events in ModeRepoPicker (first-touch / multi-repo picker).
func (m Model) handleRepoPickerMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.repoPicker == nil {
		m.mode = ModeNormal
		return m, nil
	}
	rp := m.repoPicker
	switch msg.String() {
	case KeyPickerFocus: // "enter" — same constant as KeyActivate
		if len(rp.rows) == 0 {
			return m, nil
		}
		row := rp.rows[rp.cursorIdx]
		if row.stale {
			// Stale repo: surface toast and cancel (E-MR1).
			m.repoPicker = nil
			m.mode = ModeNormal
			return m.pushToast("repo " + row.repoID + " not registered — use picker key `a` to remove it")
		}
		ticketID := rp.ticketID
		firstTouch := rp.firstTouch
		chosenID := row.repoID
		m.repoPicker = nil
		m.mode = ModeNormal

		if firstTouch {
			// Append chosen repo to assigned_repo_ids.
			if err := m.store.Mutate(func(s *state.State) error {
				return state.AddAssignment(s, ticketID, chosenID)
			}); err != nil {
				return m.pushToast("assign failed: " + err.Error())
			}
			snap, rev := m.store.Snapshot()
			m.snapshot = snap
			m.snapshotRev = rev
		}

		// Now check activations for (ticketID, chosenID).
		ps := newPickerState(m.snapshot, ticketID, chosenID)
		if ps == nil {
			activations := state.FindActivations(m.snapshot, ticketID, chosenID)
			if len(activations) == 0 {
				return m, ActivateCmd(m.ctx, m.store, m.cfg, ticketID, chosenID, "")
			}
			// 1 activation → focus directly (T-063).
			return m.focusActivation(activations[0])
		}
		// 2+ activations: open activation picker.
		m.pickerState = ps
		m.mode = ModePicker
		return m, nil

	case KeyEsc:
		m.repoPicker = nil
		m.mode = ModeNormal

	case KeyDown, "down":
		if rp.cursorIdx < len(rp.rows)-1 {
			m.repoPicker.cursorIdx++
		}
	case KeyUp, "up":
		if rp.cursorIdx > 0 {
			m.repoPicker.cursorIdx--
		}
	}
	return m, nil
}

// handleHelpMode exits on any key.
func (m Model) handleHelpMode(_ tea.KeyMsg) (Model, tea.Cmd) {
	m.mode = ModeNormal
	return m, nil
}

// ticketsForActiveCol returns the tickets in the currently active column.
// Returns nil if the board has no columns or activeColIdx is out of range.
func (m Model) ticketsForActiveCol() []state.TicketState {
	if len(m.board.Columns) == 0 || m.activeColIdx >= len(m.board.Columns) {
		return nil
	}
	return m.ticketsInCol(m.board.Columns[m.activeColIdx].ID)
}

// ticketsInCol returns tickets mapped to colID from the snapshot.
func (m Model) ticketsInCol(colID string) []state.TicketState {
	if m.snapshot == nil {
		return nil
	}
	mapped, _ := resolveTickets(m.snapshot)
	return mapped[colID]
}
