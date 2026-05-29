package ui

// TODO(M-008): replace Ticket and Column with tracker.Ticket and tracker.Column

// Ticket is a temporary placeholder type until the tracker interface is wired in M-008.
type Ticket struct {
	Key, Summary, Status string
	Labels               []string
	Priority             string
	URL                  string
	// Source distinguishes "jira" tickets from "local" ones (T-504).
	Source string
	// LocalStatus is the status for local-only tickets (T-504).
	LocalStatus string
	// IsActivating is true while an Activate goroutine is in flight for this
	// ticket. Drives the spinner badge in renderTicket and blocks new
	// activation attempts via tryActivate.
	IsActivating bool
	// ActivationCount is the number of non-removed activations (worktrees)
	// that already exist for this ticket across all repos. When >0 the card
	// shows a worktree badge and Enter focuses the existing workspace
	// instead of creating a new one.
	ActivationCount int
}

// Column is a temporary placeholder type until the tracker interface is wired in M-008.
type Column struct {
	ID, Name string
	Tickets  []Ticket
}
