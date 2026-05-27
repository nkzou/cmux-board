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
type activationDoneMsg struct {
	ActivationID string
	Err          error
}

// activationStepMsg is emitted as each side-effect step within an activation completes.
type activationStepMsg struct {
	ActivationID string
	Step         string
}

// toastExpireMsg is emitted by the re-arm tick when the soonest toast may have expired.
type toastExpireMsg struct{}
