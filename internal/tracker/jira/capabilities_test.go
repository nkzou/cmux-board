package jira

import (
	"testing"
)

// TestJiraAdapterCapabilities verifies that JiraAdapter.Capabilities() returns
// all six hardcoded flags as true. Individual field checks make regression failures obvious.
func TestJiraAdapterCapabilities(t *testing.T) {
	a := &JiraAdapter{}
	got := a.Capabilities()

	if !got.HasAssignees {
		t.Error("HasAssignees: want true, got false")
	}
	if !got.HasLabels {
		t.Error("HasLabels: want true, got false")
	}
	if !got.HasPriority {
		t.Error("HasPriority: want true, got false")
	}
	if !got.RequiresWorkflowID {
		t.Error("RequiresWorkflowID: want true, got false")
	}
	if !got.SupportsBoardSummary {
		t.Error("SupportsBoardSummary: want true, got false")
	}
	if !got.TransitionFromStatusRequired {
		t.Error("TransitionFromStatusRequired: want true, got false")
	}
}
