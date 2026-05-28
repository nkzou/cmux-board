package state

import (
	"fmt"
	"sort"
)

// AssignedRepoIDs returns a copy of the assigned_repo_ids slice for the given ticketID.
// Returns nil if the ticket does not exist or has no assignments.
// Safe to call on a Snapshot (read-only; does not mutate s).
func AssignedRepoIDs(s *State, ticketID string) []string {
	ticket, ok := s.Tickets[ticketID]
	if !ok {
		return nil
	}
	if len(ticket.AssignedRepoIDs) == 0 {
		return nil
	}
	// Return a copy so callers cannot mutate the internal slice.
	result := make([]string, len(ticket.AssignedRepoIDs))
	copy(result, ticket.AssignedRepoIDs)
	return result
}

// AddAssignment appends repoID to s.Tickets[ticketID].AssignedRepoIDs if not already
// present. It is a no-op if repoID is already in the slice.
// Preserves insertion order.
//
// MUST be called inside a store.Mutate closure; never on a snapshot.
func AddAssignment(s *State, ticketID, repoID string) error {
	if s.Tickets == nil {
		return fmt.Errorf("state.Tickets map is nil; cannot add assignment for %q", ticketID)
	}
	ticket, ok := s.Tickets[ticketID]
	if !ok {
		return fmt.Errorf("ticket %q not found in state", ticketID)
	}
	// Idempotent: skip if already present.
	for _, id := range ticket.AssignedRepoIDs {
		if id == repoID {
			return nil
		}
	}
	ticket.AssignedRepoIDs = append(ticket.AssignedRepoIDs, repoID)
	s.Tickets[ticketID] = ticket
	return nil
}

// RemoveAssignment removes repoID from s.Tickets[ticketID].AssignedRepoIDs.
// It is a no-op if repoID is not present or the ticket does not exist.
// Preserves relative order of remaining IDs.
//
// MUST be called inside a store.Mutate closure; never on a snapshot.
func RemoveAssignment(s *State, ticketID, repoID string) error {
	if s.Tickets == nil {
		return fmt.Errorf("state.Tickets map is nil; cannot remove assignment for %q", ticketID)
	}
	ticket, ok := s.Tickets[ticketID]
	if !ok {
		// Ticket absent — assignment trivially not present; treat as no-op.
		return nil
	}
	// Build a new slice excluding repoID, preserving relative order.
	filtered := make([]string, 0, len(ticket.AssignedRepoIDs))
	for _, id := range ticket.AssignedRepoIDs {
		if id != repoID {
			filtered = append(filtered, id)
		}
	}
	ticket.AssignedRepoIDs = filtered
	s.Tickets[ticketID] = ticket
	return nil
}

// ApplyAssignments replaces s.Tickets[ticketID].AssignedRepoIDs with newAssigned.
// newAssigned is the complete desired set; any previously-assigned IDs not present
// in newAssigned are dropped. Insertion order follows newAssigned (sorted by caller).
//
// MUST be called inside a store.Mutate closure; never on a snapshot.
func ApplyAssignments(s *State, ticketID string, newAssigned []string) error {
	if s.Tickets == nil {
		return fmt.Errorf("state.Tickets map is nil; cannot apply assignments for %q", ticketID)
	}
	ticket, ok := s.Tickets[ticketID]
	if !ok {
		return fmt.Errorf("ticket %q not found in state", ticketID)
	}
	// Normalize: sort to canonical order before storing.
	sorted := make([]string, len(newAssigned))
	copy(sorted, newAssigned)
	sort.Strings(sorted)
	ticket.AssignedRepoIDs = sorted
	s.Tickets[ticketID] = ticket
	return nil
}
