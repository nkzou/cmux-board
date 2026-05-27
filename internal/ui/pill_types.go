package ui

import "time"

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
// TODO(T-060): full picker overlay implementation populates this struct.
type pickerState struct {
	cursorIdx int // selected row in the picker list
}

// assignmentEditorState holds ephemeral view state for the repo assignment editor.
// Initialized on transition into ModeAssignmentEditor; nil when the editor is closed.
// TODO(T-061): full assignment editor implementation populates this struct.
type assignmentEditorState struct {
	cursorIdx int     // selected row in the repo list
	selected  []bool  // parallel to cfg.Repos slice; true == repo is assigned
}
