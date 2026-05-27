package ui

import "time"

// PollOKMsg is emitted by the poller goroutine on a successful Jira poll.
type PollOKMsg struct{ Rev uint64 }

// PollErrMsg is emitted by the poller goroutine on a poll failure.
type PollErrMsg struct {
	Code string
	When time.Time
}

// PushOKMsg is emitted when a status-transition push completes successfully.
type PushOKMsg struct {
	TicketID  string
	NewStatus string
}

// PushConflictMsg is emitted when the tracker rejects a push due to OCC mismatch.
type PushConflictMsg struct {
	TicketID     string
	ServerStatus string
}

// snapshotRefreshMsg is an internal tick that re-derives the board from the store.
type snapshotRefreshMsg struct{}

// tickPollMsg is an internal tick that initiates the next Jira poll cycle.
type tickPollMsg struct{}

// activationDoneMsg is emitted when the activation goroutine completes (success or error).
// TicketID is carried so the UI can clear the per-ticket in-flight marker
// regardless of whether activation produced a journal entry (errors before the
// first journal write yield an empty ActivationID).
type activationDoneMsg struct {
	TicketID     string
	ActivationID string
	Err          error
}

// spinnerTickMsg drives the activating-ticket spinner animation. The handler
// re-arms itself only while at least one activation is in flight.
type spinnerTickMsg struct{}

// activationStepMsg is emitted as each side-effect step within an activation completes.
type activationStepMsg struct {
	ActivationID string
	Step         string
}

// toastExpireMsg is emitted by the re-arm tick when the soonest toast may have expired.
type toastExpireMsg struct{}
