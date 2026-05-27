package state

import (
	"fmt"
	"sync"
	"testing"

	"github.com/kevin-zou/cmux-board/internal/atomicfile"
)

// openInMemory creates a Store with no backing file (uses a temp file for persistence).
func openTempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/state.json"
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

func TestMutateSnapshot(t *testing.T) {
	t.Parallel()
	s := openTempStore(t)
	err := s.Mutate(func(st *State) error {
		st.Tickets["PROJ-1"] = TicketState{Key: "PROJ-1", Summary: "hello"}
		return nil
	})
	if err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	snap, rev := s.Snapshot()
	if rev != 1 {
		t.Errorf("revision: got %d, want 1", rev)
	}
	ticket, ok := snap.Tickets["PROJ-1"]
	if !ok {
		t.Fatal("PROJ-1 not found in snapshot")
	}
	if ticket.Summary != "hello" {
		t.Errorf("Summary: got %q, want 'hello'", ticket.Summary)
	}
}

func TestMutateConcurrent(t *testing.T) {
	t.Parallel()
	s := openTempStore(t)
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("T-%d", i)
			if err := s.Mutate(func(st *State) error {
				st.Tickets[key] = TicketState{Key: key}
				return nil
			}); err != nil {
				t.Errorf("Mutate(%s): %v", key, err)
			}
		}(i)
	}
	wg.Wait()
	snap, rev := s.Snapshot()
	if len(snap.Tickets) != n {
		t.Errorf("len(Tickets): got %d, want %d", len(snap.Tickets), n)
	}
	if rev != n {
		t.Errorf("revision: got %d, want %d", rev, n)
	}
}

func TestMutateDeepCopyIsolation(t *testing.T) {
	t.Parallel()
	s := openTempStore(t)
	// Add a ticket
	if err := s.Mutate(func(st *State) error {
		st.Tickets["PROJ-1"] = TicketState{Key: "PROJ-1", Summary: "original"}
		return nil
	}); err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	// Take a snapshot and mutate it directly
	snap, _ := s.Snapshot()
	snap.Tickets["PROJ-1"] = TicketState{Key: "PROJ-1", Summary: "mutated externally"}
	// Fresh snapshot should not see the external mutation
	snap2, _ := s.Snapshot()
	if snap2.Tickets["PROJ-1"].Summary != "original" {
		t.Errorf("deep copy isolation violated: got %q, want 'original'",
			snap2.Tickets["PROJ-1"].Summary)
	}
}

func TestMutateRevisionMonotonic(t *testing.T) {
	t.Parallel()
	s := openTempStore(t)
	const n = 100
	for i := range n {
		if err := s.Mutate(func(st *State) error {
			st.SchemaVersion = 2 + i // just a mutation to force persist
			return nil
		}); err != nil {
			t.Fatalf("Mutate %d: %v", i, err)
		}
	}
	_, rev := s.Snapshot()
	if rev != n {
		t.Errorf("revision: got %d, want %d", rev, n)
	}
}

func TestOpenMissingFile(t *testing.T) {
	t.Parallel()
	s, err := Open("/nonexistent/state.json")
	if err != nil {
		t.Fatalf("Open on missing file should return no error, got: %v", err)
	}
	snap, rev := s.Snapshot()
	if rev != 0 {
		t.Errorf("revision should be 0 for new store, got %d", rev)
	}
	if snap.SchemaVersion != SchemaVersionCurrent {
		t.Errorf("SchemaVersion: got %d, want %d", snap.SchemaVersion, SchemaVersionCurrent)
	}
}

func TestOpenCorruptJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/state.json"
	if err := atomicfile.WriteFile(path, []byte(`{invalid`), 0644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	_, err := Open(path)
	if err == nil {
		t.Error("expected error for corrupt JSON state file, got nil")
	}
}
