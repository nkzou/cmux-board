package state

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDefaultState(t *testing.T) {
	t.Parallel()
	s := DefaultState()
	if s.SchemaVersion != SchemaVersionCurrent {
		t.Errorf("SchemaVersion: got %d, want %d", s.SchemaVersion, SchemaVersionCurrent)
	}
	if s.Tickets == nil {
		t.Error("Tickets map should not be nil")
	}
	if s.Activations == nil {
		t.Error("Activations map should not be nil")
	}
}

func TestDefaultState_SchemaVersion3(t *testing.T) {
	t.Parallel()
	s := DefaultState()
	if s.SchemaVersion != 3 {
		t.Errorf("SchemaVersion: got %d, want 3", s.SchemaVersion)
	}
}

func TestTicketState_NewFields(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Second)
	orig := TicketState{
		Key:           "PROJ-1",
		ID:            "10042",
		X:             5,
		Y:             7,
		Source:        "local",
		LocalStatus:   "Open",
		IssueType:     "Bug",
		AssigneeID:    "acc-1",
		AssigneeEmail: "dev@example.com",
		Priority:      "High",
		URL:           "http://x",
		Labels:        []string{"a"},
		UpdatedAt:     &now,
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got TicketState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.X != 5 {
		t.Errorf("X: got %d, want 5", got.X)
	}
	if got.Y != 7 {
		t.Errorf("Y: got %d, want 7", got.Y)
	}
	if got.Source != "local" {
		t.Errorf("Source: got %q, want 'local'", got.Source)
	}
	if got.LocalStatus != "Open" {
		t.Errorf("LocalStatus: got %q, want 'Open'", got.LocalStatus)
	}
	if got.ID != "10042" {
		t.Errorf("ID: got %q, want '10042'", got.ID)
	}
	if got.IssueType != "Bug" {
		t.Errorf("IssueType: got %q, want 'Bug'", got.IssueType)
	}
	if got.AssigneeID != "acc-1" {
		t.Errorf("AssigneeID: got %q, want 'acc-1'", got.AssigneeID)
	}
	if got.AssigneeEmail != "dev@example.com" {
		t.Errorf("AssigneeEmail: got %q, want 'dev@example.com'", got.AssigneeEmail)
	}
	if got.Priority != "High" {
		t.Errorf("Priority: got %q, want 'High'", got.Priority)
	}
	if got.URL != "http://x" {
		t.Errorf("URL: got %q, want 'http://x'", got.URL)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "a" {
		t.Errorf("Labels: got %v, want ['a']", got.Labels)
	}
	if got.UpdatedAt == nil || !got.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt: got %v, want %v", got.UpdatedAt, now)
	}
}

func TestStateJSONRoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Second)
	orig := State{
		SchemaVersion: SchemaVersionCurrent,
		Tickets: map[string]TicketState{
			"PROJ-1": {
				Key:             "PROJ-1",
				Summary:         "Test ticket",
				Status:          "In Progress",
				Source:          "jira",
				AssignedRepoIDs: []string{"repo-a"},
			},
		},
		Activations: map[string][]ActivationEntry{
			"PROJ-1": {
				{
					ActivationID: "01HZ0ABCDEFG0000000000001",
					ActIDShort:   "01hz0abc",
					RepoID:       "repo-a",
					TicketID:     "PROJ-1",
					ApproachName: "main",
					WorktreePath: "/tmp/worktrees/repo-a-PROJ-1-main",
					BranchName:   "feat/PROJ-1-main",
					ClaudeName:   "cmux-board:PROJ-1:01hz0abc",
					CmuxName:     "PROJ-1 [01hz0abc]",
					Step:         StepCmuxCreated,
					Complete:     true,
					CreatedAt:    now,
				},
			},
		},
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got State
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.SchemaVersion != orig.SchemaVersion {
		t.Errorf("SchemaVersion: got %d, want %d", got.SchemaVersion, orig.SchemaVersion)
	}
	ticket, ok := got.Tickets["PROJ-1"]
	if !ok {
		t.Fatal("PROJ-1 ticket missing after round-trip")
	}
	if ticket.Summary != "Test ticket" {
		t.Errorf("Summary: got %q, want 'Test ticket'", ticket.Summary)
	}
	acts := got.Activations["PROJ-1"]
	if len(acts) != 1 {
		t.Fatalf("expected 1 activation, got %d", len(acts))
	}
	if acts[0].ActIDShort != "01hz0abc" {
		t.Errorf("ActIDShort: got %q, want '01hz0abc'", acts[0].ActIDShort)
	}
	if !acts[0].Complete {
		t.Error("Complete should be true after round-trip")
	}
}

func TestActivationEntryStepConstants(t *testing.T) {
	t.Parallel()
	if StepStarted != "started" {
		t.Errorf("StepStarted: got %q, want 'started'", StepStarted)
	}
	if StepWorktreeCreated != "worktree_created" {
		t.Errorf("StepWorktreeCreated: got %q, want 'worktree_created'", StepWorktreeCreated)
	}
	if StepClaudeStarted != "claude_started" {
		t.Errorf("StepClaudeStarted: got %q, want 'claude_started'", StepClaudeStarted)
	}
	if StepCmuxCreated != "cmux_created" {
		t.Errorf("StepCmuxCreated: got %q, want 'cmux_created'", StepCmuxCreated)
	}
}

func TestAssignedRepoIDsPreserved(t *testing.T) {
	t.Parallel()
	orig := TicketState{
		Key:             "PROJ-2",
		Summary:         "ticket with repos",
		Status:          "To Do",
		Source:          "jira",
		AssignedRepoIDs: []string{"repo-a"},
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got TicketState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.AssignedRepoIDs) != 1 || got.AssignedRepoIDs[0] != "repo-a" {
		t.Errorf("AssignedRepoIDs not preserved: got %v, want ['repo-a']", got.AssignedRepoIDs)
	}
}
