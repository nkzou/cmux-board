package ui

// TODO(M-008): replace Ticket and Column with tracker.Ticket and tracker.Column

// Ticket is a temporary placeholder type until the tracker interface is wired in M-008.
type Ticket struct {
	Key, Summary, Status string
	Labels               []string
	Priority             string
	URL                  string
	// IsActivating is true while an Activate goroutine is in flight for this
	// ticket. Drives the spinner badge in renderTicket and blocks new
	// activation attempts via tryActivate.
	IsActivating bool
}

// Column is a temporary placeholder type until the tracker interface is wired in M-008.
type Column struct {
	ID, Name string
	Tickets  []Ticket
}
