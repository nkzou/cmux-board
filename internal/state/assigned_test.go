package state

import (
	"testing"
)

// makeTestState returns a State with the given tickets and empty Activations.
func makeTestState(tickets map[string]TicketState) *State {
	if tickets == nil {
		tickets = make(map[string]TicketState)
	}
	return &State{
		SchemaVersion: SchemaVersionCurrent,
		Tickets:       tickets,
		Activations:   make(map[string][]ActivationEntry),
	}
}

func TestAssignedRepoIDs_Empty(t *testing.T) {
	s := makeTestState(map[string]TicketState{
		"PROJ-42": {Key: "PROJ-42", AssignedRepoIDs: nil},
	})
	if got := AssignedRepoIDs(s, "PROJ-42"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestAssignedRepoIDs_NonEmpty(t *testing.T) {
	s := makeTestState(map[string]TicketState{
		"PROJ-42": {Key: "PROJ-42", AssignedRepoIDs: []string{"my-service", "cmux-board"}},
	})
	got := AssignedRepoIDs(s, "PROJ-42")
	if len(got) != 2 || got[0] != "my-service" || got[1] != "cmux-board" {
		t.Errorf("got %v, want [my-service cmux-board]", got)
	}
}

func TestAssignedRepoIDs_IsCopy(t *testing.T) {
	s := makeTestState(map[string]TicketState{
		"PROJ-42": {Key: "PROJ-42", AssignedRepoIDs: []string{"my-service"}},
	})
	got1 := AssignedRepoIDs(s, "PROJ-42")
	// Mutate the returned slice.
	got1[0] = "mutated"
	// Re-read: original should be unchanged.
	got2 := AssignedRepoIDs(s, "PROJ-42")
	if got2[0] != "my-service" {
		t.Errorf("original modified; got %v", got2)
	}
}

func TestAssignedRepoIDs_MissingTicket(t *testing.T) {
	s := makeTestState(nil)
	if got := AssignedRepoIDs(s, "NONEXISTENT"); got != nil {
		t.Errorf("expected nil for missing ticket, got %v", got)
	}
}

func TestAddAssignment_HappyPath(t *testing.T) {
	s := makeTestState(map[string]TicketState{
		"PROJ-42": {Key: "PROJ-42"},
	})
	if err := AddAssignment(s, "PROJ-42", "my-service"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := AssignedRepoIDs(s, "PROJ-42")
	if len(got) != 1 || got[0] != "my-service" {
		t.Errorf("got %v, want [my-service]", got)
	}
}

func TestAddAssignment_Idempotent(t *testing.T) {
	s := makeTestState(map[string]TicketState{
		"PROJ-42": {Key: "PROJ-42", AssignedRepoIDs: []string{"my-service"}},
	})
	if err := AddAssignment(s, "PROJ-42", "my-service"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := AssignedRepoIDs(s, "PROJ-42")
	if len(got) != 1 {
		t.Errorf("expected 1 element after idempotent add, got %v", got)
	}
}

func TestAddAssignment_InsertsOrder(t *testing.T) {
	s := makeTestState(map[string]TicketState{
		"PROJ-42": {Key: "PROJ-42"},
	})
	if err := AddAssignment(s, "PROJ-42", "repo-a"); err != nil {
		t.Fatalf("add repo-a: %v", err)
	}
	if err := AddAssignment(s, "PROJ-42", "repo-b"); err != nil {
		t.Fatalf("add repo-b: %v", err)
	}
	got := AssignedRepoIDs(s, "PROJ-42")
	if len(got) != 2 || got[0] != "repo-a" || got[1] != "repo-b" {
		t.Errorf("got %v, want [repo-a repo-b]", got)
	}
}

func TestAddAssignment_TicketAbsent(t *testing.T) {
	s := makeTestState(nil)
	err := AddAssignment(s, "NONEXISTENT", "my-service")
	if err == nil {
		t.Error("expected error for absent ticket, got nil")
	}
}

func TestRemoveAssignment_HappyPath(t *testing.T) {
	s := makeTestState(map[string]TicketState{
		"PROJ-42": {Key: "PROJ-42", AssignedRepoIDs: []string{"my-service", "cmux-board"}},
	})
	if err := RemoveAssignment(s, "PROJ-42", "my-service"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := AssignedRepoIDs(s, "PROJ-42")
	if len(got) != 1 || got[0] != "cmux-board" {
		t.Errorf("got %v, want [cmux-board]", got)
	}
}

func TestRemoveAssignment_NotPresent(t *testing.T) {
	s := makeTestState(map[string]TicketState{
		"PROJ-42": {Key: "PROJ-42", AssignedRepoIDs: []string{"my-service"}},
	})
	if err := RemoveAssignment(s, "PROJ-42", "ghost"); err != nil {
		t.Fatalf("unexpected error for absent repoID: %v", err)
	}
	got := AssignedRepoIDs(s, "PROJ-42")
	if len(got) != 1 || got[0] != "my-service" {
		t.Errorf("slice changed unexpectedly; got %v", got)
	}
}

func TestRemoveAssignment_TicketAbsent(t *testing.T) {
	s := makeTestState(nil)
	if err := RemoveAssignment(s, "NONEXISTENT", "my-service"); err != nil {
		t.Errorf("expected nil error for absent ticket, got %v", err)
	}
}

func TestRemoveAssignment_PreservesOrder(t *testing.T) {
	s := makeTestState(map[string]TicketState{
		"PROJ-42": {Key: "PROJ-42", AssignedRepoIDs: []string{"a", "b", "c"}},
	})
	if err := RemoveAssignment(s, "PROJ-42", "b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := AssignedRepoIDs(s, "PROJ-42")
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Errorf("got %v, want [a c]", got)
	}
}

func TestAddRemoveRoundTrip(t *testing.T) {
	s := makeTestState(map[string]TicketState{
		"PROJ-42": {Key: "PROJ-42"},
	})
	for _, id := range []string{"alpha", "beta", "gamma"} {
		if err := AddAssignment(s, "PROJ-42", id); err != nil {
			t.Fatalf("add %s: %v", id, err)
		}
	}
	if err := RemoveAssignment(s, "PROJ-42", "beta"); err != nil {
		t.Fatalf("remove beta: %v", err)
	}
	got := AssignedRepoIDs(s, "PROJ-42")
	if len(got) != 2 || got[0] != "alpha" || got[1] != "gamma" {
		t.Errorf("got %v, want [alpha gamma]", got)
	}
}

func TestMutateIntegration(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/state.json"

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Pre-populate a ticket.
	if err := store.Mutate(func(s *State) error {
		s.Tickets["PROJ-42"] = TicketState{Key: "PROJ-42"}
		return nil
	}); err != nil {
		t.Fatalf("Mutate (setup): %v", err)
	}

	// Add assignment inside Mutate.
	if err := store.Mutate(func(s *State) error {
		return AddAssignment(s, "PROJ-42", "my-service")
	}); err != nil {
		t.Fatalf("Mutate (AddAssignment): %v", err)
	}

	// Snapshot should reflect the assignment.
	snap, _ := store.Snapshot()
	got := AssignedRepoIDs(snap, "PROJ-42")
	if len(got) != 1 || got[0] != "my-service" {
		t.Errorf("after Mutate, got %v, want [my-service]", got)
	}
}
