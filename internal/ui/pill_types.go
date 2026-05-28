package ui

import (
	"time"

	"github.com/nkzou/cmux-board/internal/state"
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

// repoPickerRow is one row in the repo picker overlay.
type repoPickerRow struct {
	repoID      string
	displayName string
	stale       bool // true when repoID is not in cfg.Repos
}

// repoPickerState holds ephemeral view state for the repo picker overlay.
// Used for first-touch (0 assigned) and multi-repo (2+ assigned) flows.
type repoPickerState struct {
	ticketID  string
	rows      []repoPickerRow
	cursorIdx int
	// firstTouch is true when this picker is opened for a 0-assigned ticket;
	// on selection the chosen id is appended to assigned_repo_ids.
	firstTouch bool
	// manageIntent is true when the picker was opened via KeyManage; on selection
	// the activation picker opens instead of triggering activation.
	manageIntent bool
	// pendingApproach carries a name entered via ModeApproachName through repo
	// selection. When set, repo selection starts a new activation instead of
	// focusing an existing one.
	pendingApproach string
}
