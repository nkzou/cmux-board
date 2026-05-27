package state

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/kevin-zou/cmux-board/internal/atomicfile"
)

// Store serializes all state.json mutations through a single mutex.
// All callers (poller, push handler, activation orchestrator, picker actions,
// assignment editor, repos add/remove) MUST use Mutate.
// Direct mutation of *State outside the Mutate closure is forbidden.
type Store struct {
	mu       sync.Mutex
	state    *State
	path     string
	revision uint64
}

// Open loads state from path (or returns empty State if file absent).
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			initial := DefaultState()
			s.state = &initial
			return s, nil
		}
		return nil, fmt.Errorf("failed to open state: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}
	// Ensure maps are initialized even if file contained empty/omitted fields.
	if st.Tickets == nil {
		st.Tickets = make(map[string]TicketState)
	}
	if st.Activations == nil {
		st.Activations = make(map[string][]ActivationEntry)
	}
	s.state = &st
	return s, nil
}

// Mutate acquires the mutex, calls fn on a deep copy of the current state,
// and on nil return commits the copy back, increments revision, and atomically
// persists to disk. fn must not escape the *State pointer.
func (s *Store) Mutate(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := deepCopyState(s.state)
	if err := fn(cp); err != nil {
		return err
	}
	if err := persistState(s.path, cp); err != nil {
		return fmt.Errorf("failed to persist state: %w", err)
	}
	s.state = cp
	s.revision++
	return nil
}

// Snapshot returns a deep copy of the current state plus the current revision number.
// Callers may use revision for tracing but not for compare-and-swap.
func (s *Store) Snapshot() (*State, uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return deepCopyState(s.state), s.revision
}

// Flush writes the current in-memory state to disk without modifying it.
// Safe to call concurrently with Mutate (acquires the same mu lock).
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := persistState(s.path, s.state); err != nil {
		return fmt.Errorf("flush: failed to persist state: %w", err)
	}
	return nil
}

// persistState atomically writes state to disk (delegates to atomicfile.WriteFile).
func persistState(path string, st *State) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	return atomicfile.WriteFile(path, data, 0644)
}
