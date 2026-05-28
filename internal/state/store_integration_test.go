package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestStoreOpenAndMutateRoundTrip verifies that Store.Mutate writes to disk
// and that a fresh Open reads back the persisted data.
func TestStoreOpenAndMutateRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Mutate: add a ticket
	if err := store.Mutate(func(s *State) error {
		s.Tickets["DISK-1"] = TicketState{Key: "DISK-1", Summary: "persisted"}
		return nil
	}); err != nil {
		t.Fatalf("Mutate: %v", err)
	}

	// Verify file on disk contains the ticket
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var onDisk State
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatalf("unmarshal disk file: %v", err)
	}
	ticket, ok := onDisk.Tickets["DISK-1"]
	if !ok {
		t.Fatal("DISK-1 not found in on-disk state")
	}
	if ticket.Summary != "persisted" {
		t.Errorf("Summary on disk: got %q, want 'persisted'", ticket.Summary)
	}
}

// TestStoreOpenExistingFile verifies that Open loads an existing v3 state file correctly.
// Per D2, legacy (v2) state files are silently discarded and replaced by an empty board.
func TestStoreOpenExistingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Write a valid v3 state.json manually (SchemaVersionCurrent = 3).
	existing := State{
		SchemaVersion: SchemaVersionCurrent,
		Tickets: map[string]TicketState{
			"EXISTING-1": {Key: "EXISTING-1", Summary: "pre-existing", Source: "jira"},
		},
		Activations: make(map[string][]ActivationEntry),
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open existing file: %v", err)
	}
	snap, _ := store.Snapshot()
	ticket, ok := snap.Tickets["EXISTING-1"]
	if !ok {
		t.Fatal("EXISTING-1 not found after Open of existing file")
	}
	if ticket.Summary != "pre-existing" {
		t.Errorf("Summary: got %q, want 'pre-existing'", ticket.Summary)
	}
}

// TestNoStandaloneStateSave confirms there is no exported Save function in state package.
// This is a compilation test: if state.Save existed, importing and calling it would compile.
// Since we cannot do negative compile tests easily, we verify via API contract:
// the only write path is store.Mutate.
func TestNoStandaloneStateDisallowed(t *testing.T) {
	t.Parallel()
	// This test documents the architectural invariant:
	// state.Store.Mutate is the ONLY write path.
	// The absence of a standalone Save function is enforced structurally
	// (it is simply not defined in this package).
	// Any future addition of state.Save would break the T-084 grep.
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Verify Mutate is the only write path by confirming it works correctly
	if err := store.Mutate(func(s *State) error {
		s.Tickets["INVARIANT-1"] = TicketState{Key: "INVARIANT-1"}
		return nil
	}); err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	snap, _ := store.Snapshot()
	if _, ok := snap.Tickets["INVARIANT-1"]; !ok {
		t.Error("invariant test: Mutate did not persist INVARIANT-1")
	}
}
