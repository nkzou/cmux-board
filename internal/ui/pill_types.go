package ui

import (
	"time"

	"github.com/kevin-zou/cmux-board/internal/state"
)

// pillState tracks the last-emitted text and timestamp for debounce logic.
// Used by emitPill in pills.go.
type pillState struct {
	text        string
	lastEmitted time.Time
}

// toastEntry is one non-modal status message displayed at the bottom of the board.
type toastEntry struct {
	msg       string
	expiresAt time.Time
}

// pickerState holds ephemeral view state for the activation picker overlay.
// Initialized on transition into ModePicker; nil when the picker is closed.
type pickerState struct {
	ticketID  string
	repoID    string
	entries   []state.ActivationEntry // snapshot of activations for this (ticket, repo)
	cursorIdx int
}

// assignmentEditorState holds ephemeral view state for the repo assignment editor.
// Initialized on transition into ModeAssignmentEditor; nil when the editor is closed.
type assignmentEditorState struct {
	ticketID  string
	repoIDs   []string        // ordered list of all repo IDs from cfg.Repos
	checked   map[string]bool // current toggle state (pre-populated from state)
	cursorIdx int
}
