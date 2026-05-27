// Package state_test exercises Store's concurrency invariants.
// These tests MUST be run with go test -race.
package state_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/kevin-zou/cmux-board/internal/state"
	isync "github.com/kevin-zou/cmux-board/internal/sync"
	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// TestConcurrentMutateAdd50 spawns 50 goroutines that each add a distinct ticket.
// All goroutines start simultaneously via a start-gate channel to maximise contention.
// After all complete, the revision must equal 50 and all tickets must be present.
// Ties to: F17, F-NEW4, CONVENTIONS.md "State mutation discipline".
func TestConcurrentMutateAdd50(t *testing.T) {
	t.Parallel()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	const n = 50
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(n)

	for i := range n {
		go func(i int) {
			defer wg.Done()
			<-start // block until all goroutines are ready
			key := fmt.Sprintf("T-%d", i)
			if err := store.Mutate(func(s *state.State) error {
				s.Tickets[key] = state.TicketState{Key: key, Summary: "concurrent ticket"}
				return nil
			}); err != nil {
				t.Errorf("Mutate(%s): %v", key, err)
			}
		}(i)
	}

	close(start) // release all goroutines simultaneously
	wg.Wait()

	snap, rev := store.Snapshot()
	if len(snap.Tickets) != n {
		t.Errorf("len(Tickets) = %d, want %d", len(snap.Tickets), n)
	}
	if rev != n {
		t.Errorf("revision = %d, want %d", rev, n)
	}
}

// TestPollVsActivateParallelism simulates the most common real-world race condition:
// a background poller calling MergePulledTickets concurrently with an activation
// orchestrator appending an ActivationEntry. 1000 iterations, each with a per-iteration
// start-gate, plus random micro-sleeps to maximise interleaving.
// Ties to: F17, F-NEW5, Codex Finding 4, CONVENTIONS.md poll-merge contract.
func TestPollVsActivateParallelism(t *testing.T) {
	t.Parallel()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Seed with one ticket; give it a non-nil AssignedRepoIDs to verify preservation.
	seedRepos := []string{"openkanban"}
	if err := store.Mutate(func(s *state.State) error {
		s.Tickets["PROJ-1"] = state.TicketState{
			Key:             "PROJ-1",
			Summary:         "seed ticket",
			Status:          "To Do",
			LastKnownStatus: "To Do",
			AssignedRepoIDs: seedRepos,
		}
		return nil
	}); err != nil {
		t.Fatalf("seed Mutate: %v", err)
	}

	const iterations = 1000

	for iter := range iterations {
		ready := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)

		// actID is long enough to take an 8-char prefix safely.
		actID := fmt.Sprintf("act-%08d", iter)

		// Goroutine A: poller-style — merge two tickets using MergePulledTickets.
		go func() {
			defer wg.Done()
			<-ready
			// Local rng per goroutine — no shared state.
			sleep := rand.New(rand.NewSource(int64(iter*2))).Int63n(5000) //nolint:gosec
			time.Sleep(time.Duration(sleep))
			if err := store.Mutate(func(s *state.State) error {
				isync.MergePulledTickets(s, []tracker.Ticket{
					{Key: "PROJ-1", Summary: "seed ticket", Status: "To Do"},
					{Key: "PROJ-2", Summary: "new ticket", Status: "In Progress"},
				})
				return nil
			}); err != nil {
				t.Errorf("iter %d poller Mutate: %v", iter, err)
			}
		}()

		// Goroutine B: activator-style — append an ActivationEntry for PROJ-1.
		go func() {
			defer wg.Done()
			<-ready
			sleep := rand.New(rand.NewSource(int64(iter*2+1))).Int63n(5000) //nolint:gosec
			time.Sleep(time.Duration(sleep))
			if err := store.Mutate(func(s *state.State) error {
				if s.Activations == nil {
					s.Activations = make(map[string][]state.ActivationEntry)
				}
				s.Activations["PROJ-1"] = append(s.Activations["PROJ-1"], state.ActivationEntry{
					ActivationID: actID,
					ActIDShort:   actID[:8],
					TicketID:     "PROJ-1",
					RepoID:       "openkanban",
					Step:         state.StepStarted,
				})
				return nil
			}); err != nil {
				t.Errorf("iter %d activator Mutate: %v", iter, err)
			}
		}()

		close(ready) // fire both goroutines simultaneously
		wg.Wait()
	}

	snap, _ := store.Snapshot()

	// Both PROJ-1 and PROJ-2 must be present (no ticket lost by a poll overwrite).
	if _, ok := snap.Tickets["PROJ-1"]; !ok {
		t.Error("PROJ-1 missing from state after parallel runs")
	}
	if _, ok := snap.Tickets["PROJ-2"]; !ok {
		t.Error("PROJ-2 missing from state after parallel runs")
	}

	// No activation must be lost.
	if len(snap.Activations["PROJ-1"]) != iterations {
		t.Errorf("activations[PROJ-1] = %d, want %d", len(snap.Activations["PROJ-1"]), iterations)
	}

	// AssignedRepoIDs on PROJ-1 must be preserved across all polls.
	if len(snap.Tickets["PROJ-1"].AssignedRepoIDs) == 0 ||
		snap.Tickets["PROJ-1"].AssignedRepoIDs[0] != seedRepos[0] {
		t.Errorf("AssignedRepoIDs = %v, want %v", snap.Tickets["PROJ-1"].AssignedRepoIDs, seedRepos)
	}
}

// TestSnapshotIsolation verifies that mutating a snapshot's *State does NOT affect
// subsequent store.Mutate reads.
// Ties to: F17, Codex Finding 4 (deep-copy ensures callers cannot escape mutable state pointers).
func TestSnapshotIsolation(t *testing.T) {
	t.Parallel()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := store.Mutate(func(s *state.State) error {
		s.Tickets["PROJ-1"] = state.TicketState{Key: "PROJ-1", Summary: "original"}
		return nil
	}); err != nil {
		t.Fatalf("initial Mutate: %v", err)
	}

	snap1, _ := store.Snapshot()
	// Mutate snap1 directly — this should NOT affect the store.
	snap1.Tickets["PROJ-1"] = state.TicketState{Summary: "mutated"}

	// Legitimate subsequent mutation.
	if err := store.Mutate(func(s *state.State) error {
		s.Tickets["PROJ-2"] = state.TicketState{Key: "PROJ-2"}
		return nil
	}); err != nil {
		t.Fatalf("second Mutate: %v", err)
	}

	snap2, _ := store.Snapshot()

	if snap2.Tickets["PROJ-1"].Summary == "mutated" {
		t.Error("snapshot isolation violated: direct mutation of snap1 bled into snap2")
	}
	if snap2.Tickets["PROJ-1"].Summary != "original" {
		t.Errorf("PROJ-1 summary = %q, want %q", snap2.Tickets["PROJ-1"].Summary, "original")
	}
	if _, ok := snap2.Tickets["PROJ-2"]; !ok {
		t.Error("PROJ-2 missing after second Mutate")
	}
}
