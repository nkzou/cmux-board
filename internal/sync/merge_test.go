package sync

import (
	"fmt"
	"testing"
	"time"

	"github.com/kevin-zou/cmux-board/internal/state"
	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// TC-1: tracker-owned fields overwritten; local-only (assigned_repo_ids) preserved.
func TestMergePulledTickets_TrackerFieldsOverwritten(t *testing.T) {
	s := state.DefaultState()
	s.Tickets["PROJ-42"] = state.TicketState{
		Key:             "PROJ-42",
		Summary:         "old",
		AssignedRepoIDs: []string{"openkanban"},
	}

	pulled := []tracker.Ticket{
		{Key: "PROJ-42", Summary: "new", Status: "In Progress"},
	}
	MergePulledTickets(&s, pulled)

	got := s.Tickets["PROJ-42"]
	if got.Summary != "new" {
		t.Errorf("TC-1: Summary: want %q, got %q", "new", got.Summary)
	}
	if len(got.AssignedRepoIDs) != 1 || got.AssignedRepoIDs[0] != "openkanban" {
		t.Errorf("TC-1: AssignedRepoIDs: want [\"openkanban\"], got %v", got.AssignedRepoIDs)
	}
}

// TC-2: ticket removed from pulled set → removed_at stamped exactly once.
func TestMergePulledTickets_RemovedAtStampedOnce(t *testing.T) {
	s := state.DefaultState()
	s.Tickets["PROJ-42"] = state.TicketState{Key: "PROJ-42", Status: "Open"}

	// First empty pull: should stamp removed_at.
	MergePulledTickets(&s, []tracker.Ticket{})
	first := s.Tickets["PROJ-42"]
	if first.RemovedAt == nil {
		t.Fatal("TC-2: removed_at should be set after first empty pull")
	}
	firstTS := *first.RemovedAt

	// Second empty pull: should NOT overwrite removed_at.
	MergePulledTickets(&s, []tracker.Ticket{})
	second := s.Tickets["PROJ-42"]
	if second.RemovedAt == nil {
		t.Fatal("TC-2: removed_at must remain set after second empty pull")
	}
	if !second.RemovedAt.Equal(firstTS) {
		t.Errorf("TC-2: removed_at overwritten: want %v, got %v", firstTS, *second.RemovedAt)
	}
}

// TC-3: removed ticket reappears → removed_at cleared.
func TestMergePulledTickets_ReappearedTicketClearsRemovedAt(t *testing.T) {
	s := state.DefaultState()
	someTime := time.Now().Add(-time.Hour)
	s.Tickets["PROJ-42"] = state.TicketState{Key: "PROJ-42", RemovedAt: &someTime}

	pulled := []tracker.Ticket{{Key: "PROJ-42", Summary: "back", Status: "Open"}}
	MergePulledTickets(&s, pulled)

	got := s.Tickets["PROJ-42"]
	if got.RemovedAt != nil {
		t.Errorf("TC-3: removed_at should be nil after ticket reappears, got %v", got.RemovedAt)
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

// TC-5: E3 sequence — two consecutive pulls; status advances, local-only fields survive.
func TestMergePulledTickets_E3Sequence(t *testing.T) {
	s := state.DefaultState()

	// First pull: In Progress.
	MergePulledTickets(&s, []tracker.Ticket{
		{Key: "PROJ-42", Summary: "ticket", Status: "In Progress"},
	})
	// Simulate user assigning a repo.
	ts := s.Tickets["PROJ-42"]
	ts.AssignedRepoIDs = []string{"openkanban"}
	s.Tickets["PROJ-42"] = ts

	// Second pull: Done.
	MergePulledTickets(&s, []tracker.Ticket{
		{Key: "PROJ-42", Summary: "ticket", Status: "Done"},
	})

	got := s.Tickets["PROJ-42"]
	if got.LastKnownStatus != "Done" {
		t.Errorf("TC-5: LastKnownStatus: want %q, got %q", "Done", got.LastKnownStatus)
	}
	if len(got.AssignedRepoIDs) != 1 || got.AssignedRepoIDs[0] != "openkanban" {
		t.Errorf("TC-5: AssignedRepoIDs: want [\"openkanban\"], got %v", got.AssignedRepoIDs)
	}
}

// TC-6: race safety — concurrent Mutate calls must not produce data races.
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
