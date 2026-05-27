package sync

import (
	"time"

	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// MergePulledTickets updates tracker-owned fields on existing tickets and appends new
// tickets, while preserving local-only fields on existing tickets.
//
// Preserved (local-only — NEVER overwritten by a poll):
//   - assigned_repo_ids  ([]string on TicketState)
//   - removed_at         (*time.Time on TicketState; cleared if ticket reappears)
//
// Overwritten on every poll (tracker-owned):
//   - key, summary, status, assignee_id, labels, priority, url, updated_at, raw
//
// Special handling for last_known_status:
//
//	Updated to the new status ONLY if the new status differs from the prior
//	last_known_status (i.e., the tracker confirmed a transition we issued OR an
//	external actor moved the ticket). Never overwritten while a push is in flight —
//	the push handler holds its own snapshot for OCC and writes last_known_status
//	inside its own store.Mutate on success.
//
// Removed tickets (present in state.tickets but absent from pulled):
//   - Kept in state.tickets so historical activation_ids remain resolvable.
//   - removed_at is set to time.Now() THE FIRST TIME removal is observed (idempotent
//     on repeated polls — do not overwrite a non-nil removed_at with a later timestamp).
//   - removed_at is cleared to nil if the ticket reappears in a later pulled set (e.g.,
//     board filter edit or permission restore).
//
// New tickets (in pulled but not yet in state.tickets):
//   - Appended with empty assigned_repo_ids ([]string{}) and nil removed_at.
//
// The UI consumes state.tickets via a filter: entries with non-nil removed_at are hidden
// from the kanban render but remain in state for historical reference resolution.
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
				AssignedRepoIDs: []string{},
			}
		}

		// Overwrite tracker-owned fields.
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
		existing.Raw = remote.Raw

		// Update last_known_status only when the tracker's current status differs.
		if existing.LastKnownStatus != remote.Status {
			existing.LastKnownStatus = remote.Status
		}

		// Ticket reappeared: clear removed_at.
		existing.RemovedAt = nil

		// Write the modified value back into the map (value type, not pointer).
		s.Tickets[key] = existing
	}

	// Mark tickets no longer in the pulled set.
	now := time.Now()
	for key, existing := range s.Tickets {
		if _, inPulled := pulledSet[key]; !inPulled {
			if existing.RemovedAt == nil {
				// First observation of removal — stamp once.
				t := now
				existing.RemovedAt = &t
				// Write back: value type requires explicit store.
				s.Tickets[key] = existing
			}
			// Do NOT update RemovedAt if already set (preserve original removal time).
		}
	}
}
