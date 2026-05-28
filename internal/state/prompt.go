package state

import "github.com/nkzou/cmux-board/internal/tracker"

// PromptTicket converts cached state into the normalized ticket context used by
// starter_prompt templates.
func PromptTicket(t TicketState) tracker.Ticket {
	status := t.Status
	if status == "" && t.Source == "local" {
		status = t.LocalStatus
	}

	out := tracker.Ticket{
		ID:            t.ID,
		Key:           t.Key,
		Summary:       t.Summary,
		Status:        status,
		URL:           t.URL,
		IssueType:     t.IssueType,
		AssigneeID:    t.AssigneeID,
		AssigneeEmail: t.AssigneeEmail,
		Labels:        t.Labels,
		Priority:      t.Priority,
	}
	if t.UpdatedAt != nil {
		out.UpdatedAt = *t.UpdatedAt
	}
	return out
}
