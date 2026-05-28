package sync

import (
	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// MergePulledTickets updates tracker-owned fields on existing tickets and appends new
// tickets, while preserving local-only fields on existing tickets.
//
// Preserved (local-only — NEVER overwritten by a poll):
//   - assigned_repo_ids  ([]string on TicketState)
//   - x, y              (int; board position set by the user)
//
// Overwritten on every poll (tracker-owned):
//   - key, summary, status, assignee_id, labels, priority, url, updated_at
//
// Removed tickets (present in state.tickets but absent from pulled):
//   - Kept in state.tickets so historical activation_ids remain resolvable.
//   - Source is preserved so the caller can filter removed jira tickets if needed.
//
// New tickets (in pulled but not yet in state.tickets):
//   - Appended with empty assigned_repo_ids ([]string{}) and Source: "jira".
func MergePulledTickets(s *state.State, pulled []tracker.Ticket) {
	// Build a lookup set of pulled ticket keys for O(1) membership test.
	pulledSet := make(map[string]tracker.Ticket, len(pulled))
	for _, t := range pulled {
		pulledSet[t.Key] = t
	}

	// Ensure the tickets map is initialized.
	if s.Tickets == nil {
		s.Tickets = make(map[string]state.TicketState)
	}

	// Update or insert pulled tickets.
	for key, remote := range pulledSet {
		existing, ok := s.Tickets[key]
		if !ok {
			// New ticket: initialize with local-only fields at zero values.
			existing = state.TicketState{
				Source:          "jira",
				AssignedRepoIDs: []string{},
			}
		}

		// Overwrite tracker-owned fields; preserve local-only fields (X, Y, Source).
		existing.Key = remote.Key
		existing.Summary = remote.Summary
		existing.Status = remote.Status
		existing.AssigneeID = remote.AssigneeID
		existing.Labels = remote.Labels
		existing.Priority = remote.Priority
		existing.URL = remote.URL
		if !remote.UpdatedAt.IsZero() {
			t := remote.UpdatedAt
			existing.UpdatedAt = &t
		}

		// Write the modified value back into the map (value type, not pointer).
		s.Tickets[key] = existing
	}
}
