package refresh

import "time"

// PollErrorKind distinguishes the two poll error categories for pill-text routing.
type PollErrorKind int

const (
	// PollErrorAuth is a 401 Unauthorized — credentials expired or revoked.
	PollErrorAuth PollErrorKind = iota
	// PollErrorTransient covers 5xx, network errors, and other retryable failures.
	PollErrorTransient
)

// pillTextAuth is the exact string required by E5 (locked — do not change).
const pillTextAuth = "re-auth required — run cmux-board init --force"

// PillText returns the status-pill display string for this error kind.
// The UI passes this to cmuxcli.SetStatus("tracker", ...).
func PillText(kind PollErrorKind, at time.Time) string {
	switch kind {
	case PollErrorAuth:
		return pillTextAuth
	default:
		return "offline (since " + at.Format("15:04") + ")"
	}
}
