package ui

// TODO(M-008): replace Ticket and Column with tracker.Ticket and tracker.Column

// Ticket is a temporary placeholder type until the tracker interface is wired in M-008.
type Ticket struct {
	Key, Summary, Status string
	Labels               []string
	Priority             string
	URL                  string
}

// Column is a temporary placeholder type until the tracker interface is wired in M-008.
type Column struct {
	ID, Name string
	Tickets  []Ticket
}
