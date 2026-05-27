package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kevin-zou/cmux-board/internal/state"
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
		// TODO(T-064): resolveRepoForActivation — emit tea.Cmd that calls activate.Activate.
		return m, nil
	case KeyNewApproach:
		// Capital N: new approach regardless of existing activations (F16).
		// MUST NOT check len(activations) before entering ModeApproachName.
		m.previousMode = ModeNormal
		m.mode = ModeApproachName
		m.approachNameInput.SetValue("")
		m.approachNameInput.Focus()
	case KeyAssignRepos:
		// TODO(T-061): initialize assignmentEditor from current ticket + cfg.Repos.
		m.assignmentEditor = &assignmentEditorState{}
		m.mode = ModeAssignmentEditor
	case KeyFilter:
		// TODO(T-066): focus filter input.
		m.mode = ModeFilter
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
	switch msg.String() {
	case KeyPickerFocus:
		// TODO(T-063): call focus.Focus on selected activation.
		return m, nil
	case KeyPickerNew:
		m.previousMode = ModePicker
		m.mode = ModeApproachName
		m.approachNameInput.SetValue("")
		m.approachNameInput.Focus()
	case KeyPickerRespawn:
		// TODO(T-063): respawn selected activation.
		return m, nil
	case KeyPickerDelete:
		// Delete activation cache entry only — does NOT call any cmux or claude destructor.
		// store.Mutate removes the entry from state.activations.
		// TODO(T-057): implement delete via store.Mutate when activation is selected.
		return m, nil
	case KeyPickerCancel: // == KeyEsc == "esc"
		m.pickerState = nil
		m.mode = ModeNormal
	case KeyDown, "down":
		// picker cursor movement; clamped by T-060 once activation list length is known.
		if m.pickerState.cursorIdx < 0 {
			m.pickerState.cursorIdx = 0
		} else {
			m.pickerState.cursorIdx++
		}
	case KeyUp, "up":
		if m.pickerState.cursorIdx > 0 {
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
		// TODO(T-062): emit tea.Cmd that calls activate.Activate with name.
		return m, nil
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

// handleAssignmentEditorMode is a stub. Full implementation in T-061.
func (m Model) handleAssignmentEditorMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == KeyAssignCancel {
		m.assignmentEditor = nil
		m.mode = ModeNormal
	}
	return m, nil
}

// handleFilterMode is a stub. Full implementation in T-066.
func (m Model) handleFilterMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == KeyEsc {
		m.mode = ModeNormal
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
