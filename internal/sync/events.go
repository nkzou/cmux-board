package sync

import "time"

// PollErrorKind distinguishes the two poll error categories for pill-text routing.
type PollErrorKind int

const (
	// PollErrorAuth is a 401 Unauthorized — credentials expired or revoked.
	PollErrorAuth PollErrorKind = iota
	// PollErrorTransient covers 5xx, network errors, and other retryable failures.
	PollErrorTransient
)

// PollOKMsg is emitted by the poller after a successful pull.
type PollOKMsg struct {
	At time.Time
}

// PollErrMsg is emitted by the poller after a failed pull.
type PollErrMsg struct {
	Kind PollErrorKind
	At   time.Time
}

// pillTextAuth is the exact string required by E5 (locked — do not change).
const pillTextAuth = "re-auth required — run cmux-board init --force"

// PillText returns the status-pill display string for this error.
// The UI passes this to cmuxcli.SetStatus("tracker", ...).
func (m PollErrMsg) PillText() string {
	switch m.Kind {
	case PollErrorAuth:
		return pillTextAuth
	default:
		return "offline (since " + m.At.Format("15:04") + ")"
	}
}

// ConflictKind distinguishes the two push-conflict cases.
// Declared here (not in push.go) so that PushConflictMsg can reference it without a circular dependency.
type ConflictKind int

const (
	ConflictOCC              ConflictKind = iota // tracker.ErrConflict
	ConflictInvalidTransition                    // tracker.ErrInvalidTransition (RT-3)
)

// PushOKMsg is emitted by the push handler after a successful transition.
type PushOKMsg struct {
	TicketID  string
	NewStatus string
}

// PushConflictMsg is emitted when the push handler returns Result{Conflict: true}.
// The UI uses ServerStatus to snap the card back to the correct column.
type PushConflictMsg struct {
	TicketID     string
	ServerStatus string
	Kind         ConflictKind // ConflictOCC or ConflictInvalidTransition (RT-3)
}
