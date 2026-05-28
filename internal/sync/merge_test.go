package sync

import (
	"fmt"
	"testing"

	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// TC-1: tracker-owned fields overwritten; local-only (assigned_repo_ids, x, y) preserved.
func TestMergePulledTickets_TrackerFieldsOverwritten(t *testing.T) {
	s := state.DefaultState()
	s.Tickets["PROJ-42"] = state.TicketState{
		Key:             "PROJ-42",
		Summary:         "old",
		X:               3,
		Y:               5,
		AssignedRepoIDs: []string{"my-service"},
	}

	pulled := []tracker.Ticket{
		{Key: "PROJ-42", Summary: "new", Status: "In Progress"},
	}
	MergePulledTickets(&s, pulled)

	got := s.Tickets["PROJ-42"]
	if got.Summary != "new" {
		t.Errorf("TC-1: Summary: want %q, got %q", "new", got.Summary)
	}
	if len(got.AssignedRepoIDs) != 1 || got.AssignedRepoIDs[0] != "my-service" {
		t.Errorf("TC-1: AssignedRepoIDs: want [\"my-service\"], got %v", got.AssignedRepoIDs)
	}
	// Local position must be preserved across poll.
	if got.X != 3 || got.Y != 5 {
		t.Errorf("TC-1: (X,Y): want (3,5), got (%d,%d)", got.X, got.Y)
	}
}

// TC-2: ticket absent from pulled set is kept in state (historical resolution).
func TestMergePulledTickets_AbsentTicketRetained(t *testing.T) {
	s := state.DefaultState()
	s.Tickets["PROJ-42"] = state.TicketState{Key: "PROJ-42", Status: "Open"}

	// Empty pull — PROJ-42 absent.
	MergePulledTickets(&s, []tracker.Ticket{})

	if _, ok := s.Tickets["PROJ-42"]; !ok {
		t.Error("TC-2: absent ticket should be retained in state for historical resolution")
	}
}

// TC-3: ticket reappears after absence — tracker-owned fields updated.
func TestMergePulledTickets_ReappearedTicketUpdated(t *testing.T) {
	s := state.DefaultState()
	s.Tickets["PROJ-42"] = state.TicketState{Key: "PROJ-42", Status: "Open", X: 2, Y: 4}

	// Empty pull — PROJ-42 not in pulled set.
	MergePulledTickets(&s, []tracker.Ticket{})

	// Ticket reappears with new summary/status.
	pulled := []tracker.Ticket{{Key: "PROJ-42", Summary: "back", Status: "In Progress"}}
	MergePulledTickets(&s, pulled)

	got := s.Tickets["PROJ-42"]
	if got.Summary != "back" {
		t.Errorf("TC-3: Summary: want %q, got %q", "back", got.Summary)
	}
	if got.Status != "In Progress" {
		t.Errorf("TC-3: Status: want %q, got %q", "In Progress", got.Status)
	}
	// Position preserved across re-appearance.
	if got.X != 2 || got.Y != 4 {
		t.Errorf("TC-3: (X,Y): want (2,4), got (%d,%d)", got.X, got.Y)
	}
}

// TC-4: new ticket appended with empty (non-nil) assigned_repo_ids.
func TestMergePulledTickets_NewTicketEmptyRepoIDs(t *testing.T) {
	s := state.DefaultState()
	s.Tickets = make(map[string]state.TicketState)

	pulled := []tracker.Ticket{{Key: "PROJ-99", Summary: "brand new", Status: "Open"}}
	MergePulledTickets(&s, pulled)

	got, ok := s.Tickets["PROJ-99"]
	if !ok {
		t.Fatal("TC-4: PROJ-99 not found in state after merge")
	}
	if got.AssignedRepoIDs == nil {
		t.Error("TC-4: AssignedRepoIDs must be non-nil empty slice, got nil")
	}
	if len(got.AssignedRepoIDs) != 0 {
		t.Errorf("TC-4: AssignedRepoIDs must be empty, got %v", got.AssignedRepoIDs)
	}
}

// TC-5: E3 sequence -- two consecutive pulls; status advances, local-only fields survive.
func TestMergePulledTickets_E3Sequence(t *testing.T) {
	s := state.DefaultState()

	// First pull: In Progress.
	MergePulledTickets(&s, []tracker.Ticket{
		{Key: "PROJ-42", Summary: "ticket", Status: "In Progress"},
	})
	// Simulate user assigning a repo.
	ts := s.Tickets["PROJ-42"]
	ts.AssignedRepoIDs = []string{"my-service"}
	s.Tickets["PROJ-42"] = ts

	// Second pull: Done.
	MergePulledTickets(&s, []tracker.Ticket{
		{Key: "PROJ-42", Summary: "ticket", Status: "Done"},
	})

	got := s.Tickets["PROJ-42"]
	if got.Status != "Done" {
		t.Errorf("TC-5: Status: want %q, got %q", "Done", got.Status)
	}
	if len(got.AssignedRepoIDs) != 1 || got.AssignedRepoIDs[0] != "my-service" {
		t.Errorf("TC-5: AssignedRepoIDs: want [\"my-service\"], got %v", got.AssignedRepoIDs)
	}
}

// TC-6: race safety -- concurrent Mutate calls must not produce data races.
func TestMergePulledTickets_RaceSafety(t *testing.T) {
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("TC-6: failed to open store: %v", err)
	}

	const workers = 5
	done := make(chan struct{}, workers)
	for i := 0; i < workers; i++ {
		key := fmt.Sprintf("T-%d", i)
		go func(k string) {
			defer func() { done <- struct{}{} }()
			_ = store.Mutate(func(s *state.State) error {
				MergePulledTickets(s, []tracker.Ticket{
					{Key: k, Summary: "concurrent", Status: "Open"},
				})
				return nil
			})
		}(key)
	}
	for i := 0; i < workers; i++ {
		<-done
	}

	snap, rev := store.Snapshot()
	if len(snap.Tickets) != workers {
		t.Errorf("TC-6: want %d tickets, got %d", workers, len(snap.Tickets))
	}
	if rev != workers {
		t.Errorf("TC-6: want revision %d, got %d", workers, rev)
	}
}
