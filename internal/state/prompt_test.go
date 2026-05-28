package state

import (
	"testing"
	"time"
)

func TestPromptTicket_UsesCachedJiraFields(t *testing.T) {
	t.Parallel()
	updatedAt := time.Date(2026, 5, 28, 14, 0, 0, 0, time.UTC)
	got := PromptTicket(TicketState{
		Key:           "PROJ-1",
		ID:            "10001",
		Source:        "jira",
		Summary:       "Fix login",
		Status:        "In Progress",
		IssueType:     "Bug",
		Priority:      "High",
		AssigneeID:    "acc-1",
		AssigneeEmail: "dev@example.com",
		URL:           "https://jira.example.com/browse/PROJ-1",
		Labels:        []string{"auth", "backend"},
		UpdatedAt:     &updatedAt,
	})

	if got.ID != "10001" || got.Key != "PROJ-1" || got.Summary != "Fix login" {
		t.Fatalf("basic fields not copied: %+v", got)
	}
	if got.Status != "In Progress" || got.IssueType != "Bug" || got.Priority != "High" {
		t.Fatalf("jira fields not copied: %+v", got)
	}
	if got.AssigneeID != "acc-1" || got.AssigneeEmail != "dev@example.com" {
		t.Fatalf("assignee fields not copied: %+v", got)
	}
	if got.URL != "https://jira.example.com/browse/PROJ-1" {
		t.Fatalf("URL = %q", got.URL)
	}
	if len(got.Labels) != 2 || got.Labels[0] != "auth" || got.Labels[1] != "backend" {
		t.Fatalf("Labels = %v", got.Labels)
	}
	if !got.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("UpdatedAt = %v, want %v", got.UpdatedAt, updatedAt)
	}
}

func TestPromptTicket_LocalTicketUsesLocalStatus(t *testing.T) {
	t.Parallel()
	got := PromptTicket(TicketState{
		Key:         "LOCAL-1",
		Source:      "local",
		Summary:     "Scratch note",
		LocalStatus: "Open",
	})

	if got.Status != "Open" {
		t.Fatalf("Status = %q, want LocalStatus", got.Status)
	}
}
