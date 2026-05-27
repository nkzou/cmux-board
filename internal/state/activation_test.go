package state

import (
	"testing"
	"time"
)

func timePtr() *time.Time {
	t := time.Now()
	return &t
}

func makeActState(activations map[string][]ActivationEntry) *State {
	if activations == nil {
		activations = make(map[string][]ActivationEntry)
	}
	return &State{
		SchemaVersion: SchemaVersionCurrent,
		Tickets:       make(map[string]TicketState),
		Activations:   activations,
	}
}

func TestFindActivations_Zero(t *testing.T) {
	s := makeActState(nil)
	got := FindActivations(s, "PROJ-42", "my-service")
	if got == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 entries, got %d", len(got))
	}
}

func TestFindActivations_One(t *testing.T) {
	s := makeActState(map[string][]ActivationEntry{
		"PROJ-42": {{ActivationID: "act-1", RepoID: "my-service", TicketID: "PROJ-42"}},
	})
	got := FindActivations(s, "PROJ-42", "my-service")
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].ActivationID != "act-1" {
		t.Errorf("ActivationID = %q, want 'act-1'", got[0].ActivationID)
	}
}

func TestFindActivations_Multiple(t *testing.T) {
	s := makeActState(map[string][]ActivationEntry{
		"PROJ-42": {
			{ActivationID: "act-1", RepoID: "my-service"},
			{ActivationID: "act-2", RepoID: "my-service"},
			{ActivationID: "act-3", RepoID: "cmux-board"},
		},
	})
	got := FindActivations(s, "PROJ-42", "my-service")
	if len(got) != 2 {
		t.Fatalf("expected 2 entries for my-service, got %d", len(got))
	}
}

func TestFindActivations_RepoFilter(t *testing.T) {
	s := makeActState(map[string][]ActivationEntry{
		"PROJ-42": {
			{ActivationID: "act-1", RepoID: "repo-a"},
			{ActivationID: "act-2", RepoID: "repo-b"},
		},
	})
	got := FindActivations(s, "PROJ-42", "repo-a")
	if len(got) != 1 || got[0].RepoID != "repo-a" {
		t.Errorf("expected single repo-a entry, got %v", got)
	}
}

func TestFindActivationByID_Hit(t *testing.T) {
	s := makeActState(map[string][]ActivationEntry{
		"PROJ-42": {{ActivationID: "act-1", RepoID: "my-service"}},
		"PROJ-99": {{ActivationID: "act-2", RepoID: "my-service"}},
	})
	got, ok := FindActivationByID(s, "act-2")
	if !ok {
		t.Fatal("expected hit for act-2")
	}
	if got.ActivationID != "act-2" {
		t.Errorf("got %q, want 'act-2'", got.ActivationID)
	}
}

func TestFindActivationByID_Miss(t *testing.T) {
	s := makeActState(map[string][]ActivationEntry{
		"PROJ-42": {{ActivationID: "act-1"}},
	})
	got, ok := FindActivationByID(s, "ghost")
	if ok {
		t.Errorf("expected miss, got entry %+v", got)
	}
	if got != nil {
		t.Error("expected nil pointer on miss")
	}
}

func TestFindActivationByShortID_Hit(t *testing.T) {
	s := makeActState(map[string][]ActivationEntry{
		"PROJ-42": {{ActivationID: "act-full-1", ActIDShort: "abc12345"}},
		"PROJ-99": {{ActivationID: "act-full-2", ActIDShort: "def67890"}},
	})
	got, ok := FindActivationByShortID(s, "def67890")
	if !ok {
		t.Fatal("expected hit for def67890")
	}
	if got.ActivationID != "act-full-2" {
		t.Errorf("got %q, want 'act-full-2'", got.ActivationID)
	}
}

func TestFindActivationByShortID_Miss(t *testing.T) {
	s := makeActState(map[string][]ActivationEntry{
		"PROJ-42": {{ActIDShort: "abc12345"}},
	})
	_, ok := FindActivationByShortID(s, "00000000")
	if ok {
		t.Error("expected miss for 00000000")
	}
}

func TestAllIncompleteActivations(t *testing.T) {
	s := makeActState(map[string][]ActivationEntry{
		"PROJ-42": {
			{ActivationID: "complete-1", Complete: true},
			{ActivationID: "incomplete-1", Complete: false},
		},
		"PROJ-99": {
			{ActivationID: "incomplete-2", Complete: false},
		},
	})
	got := AllIncompleteActivations(s)
	if len(got) != 2 {
		t.Fatalf("expected 2 incomplete entries, got %d", len(got))
	}
	for _, act := range got {
		if act.Complete {
			t.Errorf("AllIncompleteActivations returned a complete entry: %+v", act)
		}
	}
}

func TestAllIncompleteActivations_AllComplete(t *testing.T) {
	s := makeActState(map[string][]ActivationEntry{
		"PROJ-42": {
			{ActivationID: "complete-1", Complete: true},
		},
	})
	got := AllIncompleteActivations(s)
	if len(got) != 0 {
		t.Errorf("expected 0 entries, got %d", len(got))
	}
}

func TestAllIncompleteActivations_NotNil(t *testing.T) {
	s := makeActState(nil)
	got := AllIncompleteActivations(s)
	if got == nil {
		t.Error("expected non-nil empty slice")
	}
}

func TestActivationCount(t *testing.T) {
	now := timePtr()
	cases := []struct {
		name        string
		activations map[string][]ActivationEntry
		ticket      string
		want        int
	}{
		{
			name:        "no entries returns 0",
			activations: nil,
			ticket:      "PROJ-42",
			want:        0,
		},
		{
			name: "single active counts",
			activations: map[string][]ActivationEntry{
				"PROJ-42": {{ActivationID: "a1", RepoID: "r1"}},
			},
			ticket: "PROJ-42",
			want:   1,
		},
		{
			name: "multiple across repos all count",
			activations: map[string][]ActivationEntry{
				"PROJ-42": {
					{ActivationID: "a1", RepoID: "r1"},
					{ActivationID: "a2", RepoID: "r2"},
				},
			},
			ticket: "PROJ-42",
			want:   2,
		},
		{
			name: "removed entries are excluded",
			activations: map[string][]ActivationEntry{
				"PROJ-42": {
					{ActivationID: "a1", RepoID: "r1"},
					{ActivationID: "a2", RepoID: "r2", RemovedAt: now},
				},
			},
			ticket: "PROJ-42",
			want:   1,
		},
		{
			name: "other tickets do not bleed",
			activations: map[string][]ActivationEntry{
				"PROJ-42": {{ActivationID: "a1"}},
				"PROJ-99": {{ActivationID: "b1"}, {ActivationID: "b2"}},
			},
			ticket: "PROJ-99",
			want:   2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := makeActState(tc.activations)
			got := ActivationCount(s, tc.ticket)
			if got != tc.want {
				t.Errorf("ActivationCount(%q) = %d, want %d", tc.ticket, got, tc.want)
			}
		})
	}
}
