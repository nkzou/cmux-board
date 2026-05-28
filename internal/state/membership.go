package state

import (
	"github.com/oklog/ulid/v2"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// ImportJiraTicket upserts a tracker-sourced ticket into s.Tickets.
// If the ticket is absent, it is inserted with Source: "jira", X: 0, Y: 0, and
// AssignedRepoIDs initialized to an empty (non-nil) slice.
// If the ticket is already present, tracker-owned fields (Summary, Status, URL,
// Labels, Priority, AssigneeID) are updated; local-only fields (X, Y, Source,
// AssignedRepoIDs) are preserved.
//
// MUST be called inside a Store.Mutate closure. Direct invocation on a *State
// obtained from Store.Snapshot() is a race and will not persist; the deep-copy
// returned by Snapshot is intentionally disposable.
func ImportJiraTicket(s *State, t tracker.Ticket) {
	existing, ok := s.Tickets[t.Key]
	if !ok {
		existing = TicketState{
			Key:             t.Key,
			Source:          "jira",
			X:               0,
			Y:               0,
			AssignedRepoIDs: []string{},
		}
	}
	// Overwrite tracker-owned fields; preserve local-only (X, Y, Source, AssignedRepoIDs).
	existing.Summary = t.Summary
	existing.Status = t.Status
	existing.URL = t.URL
	existing.Labels = t.Labels
	existing.Priority = t.Priority
	existing.AssigneeID = t.AssigneeID
	s.Tickets[t.Key] = existing
}

// CreateLocalTicket inserts a local-only ticket with a fresh ULID-derived key.
// Returns the new key. The ticket is initialized with LocalStatus: "Open" and X: 0, Y: 0.
//
// MUST be called inside a Store.Mutate closure. Direct invocation on a *State
// obtained from Store.Snapshot() is a race and will not persist; the deep-copy
// returned by Snapshot is intentionally disposable.
func CreateLocalTicket(s *State, name string) string {
	key := "LOCAL-" + ulid.Make().String()
	s.Tickets[key] = TicketState{
		Key:         key,
		Source:      "local",
		Summary:     name,
		LocalStatus: "Open",
		X:           0,
		Y:           0,
	}
	return key
}

// RemoveTicket deletes the ticket identified by key from s.Tickets.
// It is a no-op when key is absent.
//
// MUST be called inside a Store.Mutate closure. Direct invocation on a *State
// obtained from Store.Snapshot() is a race and will not persist; the deep-copy
// returned by Snapshot is intentionally disposable.
func RemoveTicket(s *State, key string) {
	delete(s.Tickets, key)
}

// SetTicketPosition updates the (X, Y) board coordinates for the ticket identified
// by key. It is a no-op when key is absent.
//
// MUST be called inside a Store.Mutate closure. Direct invocation on a *State
// obtained from Store.Snapshot() is a race and will not persist; the deep-copy
// returned by Snapshot is intentionally disposable.
func SetTicketPosition(s *State, key string, x, y int) {
	t, ok := s.Tickets[key]
	if !ok {
		return
	}
	t.X = x
	t.Y = y
	s.Tickets[key] = t
}

// cycleStatuses defines the rotation order for CycleStatus on local tickets.
var cycleStatuses = []string{"Open", "In Progress", "Done"}

// CycleStatus rotates the status of the ticket one step forward:
//   - Local tickets (Source == "local"): rotates LocalStatus through Open → In Progress → Done → Open.
//   - Jira tickets (Source == "jira"):   rotates Status through Open → In Progress → Done → Open.
//
// The (X, Y) coordinates are unchanged (F6 invariant).
// It is a no-op when key is absent.
//
// MUST be called inside a Store.Mutate closure. Direct invocation on a *State
// obtained from Store.Snapshot() is a race and will not persist; the deep-copy
// returned by Snapshot is intentionally disposable.
func CycleStatus(s *State, key string) {
	t, ok := s.Tickets[key]
	if !ok {
		return
	}
	if t.Source == "local" {
		t.LocalStatus = nextCycleStatus(t.LocalStatus)
	} else {
		t.Status = nextCycleStatus(t.Status)
	}
	s.Tickets[key] = t
}

// nextCycleStatus returns the next status in the rotation cycle.
// Unknown values start the cycle from the beginning.
func nextCycleStatus(current string) string {
	for i, s := range cycleStatuses {
		if s == current {
			return cycleStatuses[(i+1)%len(cycleStatuses)]
		}
	}
	// Unknown status: start from beginning.
	return cycleStatuses[0]
}
