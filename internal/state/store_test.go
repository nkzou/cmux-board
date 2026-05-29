package state

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/nkzou/cmux-board/internal/atomicfile"
)

// openTempStore creates a Store with no backing file (uses a temp file for persistence).
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
			st.SchemaVersion = 3 + i // just a mutation to force persist
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

// TestOpen_LegacyFileBecomesEmptyBoard verifies that a JSON file with a legacy schema
// version (e.g. v2) is silently discarded and replaced by DefaultState. No backup file
// is created for legacy schema — it is a clean, expected migration path (E5, D2).
func TestOpen_LegacyFileBecomesEmptyBoard(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/state.json"
	// Write a valid v2 state file.
	if err := atomicfile.WriteFile(path, []byte(`{"schema_version":2,"tickets":{"PROJ-1":{"key":"PROJ-1","summary":"old"}}}`), 0644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open with legacy file should not error: %v", err)
	}
	snap, _ := s.Snapshot()

	if snap.SchemaVersion != SchemaVersionCurrent {
		t.Errorf("SchemaVersion: got %d, want %d", snap.SchemaVersion, SchemaVersionCurrent)
	}
	if len(snap.Tickets) != 0 {
		t.Errorf("legacy state should produce empty board, got %d tickets", len(snap.Tickets))
	}

	// No backup sibling should be created for a legacy schema (only for corruption).
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".corrupt-") {
			t.Errorf("unexpected backup file for legacy schema: %q", e.Name())
		}
	}
}

// TestOpen_CorruptJSONPreservesOriginal verifies that a truncated/malformed JSON file is
// preserved as a .corrupt-* sibling and replaced by DefaultState. No data is silently lost
// (Review F-05 data-safety requirement).
func TestOpen_CorruptJSONPreservesOriginal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/state.json"
	original := []byte(`{"schema_v`) // truncated — not valid JSON
	if err := atomicfile.WriteFile(path, original, 0644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open with corrupt file should not error (soft fallback): %v", err)
	}
	snap, _ := s.Snapshot()

	if snap.SchemaVersion != SchemaVersionCurrent {
		t.Errorf("SchemaVersion: got %d, want %d", snap.SchemaVersion, SchemaVersionCurrent)
	}
	if len(snap.Tickets) != 0 {
		t.Errorf("corrupt state should produce empty board, got %d tickets", len(snap.Tickets))
	}

	// Exactly one .corrupt-* sibling must exist and must contain the original bytes.
	entries, _ := os.ReadDir(dir)
	var backupFiles []os.DirEntry
	for _, e := range entries {
		if strings.Contains(e.Name(), ".corrupt-") {
			backupFiles = append(backupFiles, e)
		}
	}
	if len(backupFiles) != 1 {
		t.Fatalf("expected exactly 1 .corrupt-* backup file, got %d", len(backupFiles))
	}
	backupPath := dir + "/" + backupFiles[0].Name()
	got, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("backup contents mismatch: got %q, want %q", got, original)
	}
}

// TestOpen_CurrentSchemaRoundTrips verifies that a valid v3 state file is loaded intact.
func TestOpen_CurrentSchemaRoundTrips(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/state.json"

	// Create a valid v3 state.
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("Open (create): %v", err)
	}
	if err := s1.Mutate(func(st *State) error {
		st.Tickets["PROJ-1"] = TicketState{
			Key:     "PROJ-1",
			Summary: "hello",
			Source:  "jira",
			X:       5,
			Y:       7,
		}
		return nil
	}); err != nil {
		t.Fatalf("Mutate: %v", err)
	}

	// Re-open and verify the ticket is preserved.
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open (reload): %v", err)
	}
	snap, _ := s2.Snapshot()
	ticket, ok := snap.Tickets["PROJ-1"]
	if !ok {
		t.Fatal("PROJ-1 missing after reload")
	}
	if ticket.Summary != "hello" {
		t.Errorf("Summary: got %q, want 'hello'", ticket.Summary)
	}
	if ticket.X != 5 || ticket.Y != 7 {
		t.Errorf("(X, Y): got (%d, %d), want (5, 7)", ticket.X, ticket.Y)
	}
}
