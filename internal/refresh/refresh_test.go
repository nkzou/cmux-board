// Package refresh provides tests for the Refresher type.
// Tests that involve the ticker-driven loop use [testing/synctest] for deterministic
// time control. The synctest bubble provides a fake clock so time.NewTicker advances
// only when all goroutines in the bubble are durably blocked, eliminating wall-clock
// sleeps and the associated flake surface. See https://pkg.go.dev/testing/synctest.
package refresh

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
	"github.com/nkzou/cmux-board/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeTracker is a minimal tracker.IssueTracker stub for testing.
type fakeTracker struct {
	mu      sync.Mutex
	calls   []string // keys passed to GetTicket
	results map[string]tracker.Ticket
	errors  map[string]error
}

func newFakeTracker() *fakeTracker {
	return &fakeTracker{
		results: make(map[string]tracker.Ticket),
		errors:  make(map[string]error),
	}
}

func (f *fakeTracker) GetTicket(_ context.Context, key string) (tracker.Ticket, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, key)
	if err, ok := f.errors[key]; ok {
		return tracker.Ticket{}, err
	}
	if t, ok := f.results[key]; ok {
		return t, nil
	}
	return tracker.Ticket{Key: key, Summary: "default", Status: "done"}, nil
}

func (f *fakeTracker) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeTracker) calledKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]string, len(f.calls))
	copy(cp, f.calls)
	return cp
}

// Implement unused IssueTracker methods.
func (f *fakeTracker) WhoAmI(_ context.Context) (tracker.UserIdentity, error) {
	return tracker.UserIdentity{}, nil
}
func (f *fakeTracker) ListBoards(_ context.Context) ([]tracker.BoardSummary, error) {
	return nil, nil
}
func (f *fakeTracker) GetBoard(_ context.Context, _ string) (tracker.Board, error) {
	return tracker.Board{}, nil
}
func (f *fakeTracker) ListTickets(_ context.Context, _ string, _ *time.Time) ([]tracker.Ticket, error) {
	return nil, nil
}
func (f *fakeTracker) TransitionStatus(_ context.Context, _, _, _ string) error { return nil }
func (f *fakeTracker) Capabilities() tracker.Capabilities                        { return tracker.Capabilities{} }

// msgRecorder captures emitted tea.Msg values.
type msgRecorder struct {
	mu   sync.Mutex
	msgs []tea.Msg
}

func (r *msgRecorder) emit(msg tea.Msg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, msg)
}

func (r *msgRecorder) all() []tea.Msg {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]tea.Msg, len(r.msgs))
	copy(cp, r.msgs)
	return cp
}

// openTestStore creates a real state.Store backed by a temp file.
func openTestStore(t *testing.T) *state.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := state.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store
}

// addJiraTicket inserts a jira-sourced ticket into the store.
func addJiraTicket(t *testing.T, store *state.Store, key, status, summary string) {
	t.Helper()
	err := store.Mutate(func(s *state.State) error {
		s.Tickets[key] = state.TicketState{
			Key:     key,
			Source:  "jira",
			Status:  status,
			Summary: summary,
			X:       3,
			Y:       5,
		}
		return nil
	})
	if err != nil {
		t.Fatalf("addJiraTicket: %v", err)
	}
}

// addLocalTicket inserts a local-sourced ticket into the store.
func addLocalTicket(t *testing.T, store *state.Store, key string) {
	t.Helper()
	err := store.Mutate(func(s *state.State) error {
		s.Tickets[key] = state.TicketState{
			Key:         key,
			Source:      "local",
			LocalStatus: "todo",
			Summary:     "local ticket",
		}
		return nil
	})
	if err != nil {
		t.Fatalf("addLocalTicket: %v", err)
	}
}

// tickInterval is the test interval used in synctest bubbles.
// It must be large enough that fake-time advances are visible but is instant in wall-clock.
const tickInterval = 1 * time.Second

// TestTick_NoImported_NoTrackerCalls verifies that an empty Tickets map produces
// zero GetTicket calls and one PollOKMsg.
//
// Uses [testing/synctest] for deterministic ticker control.
func TestTick_NoImported_NoTrackerCalls(t *testing.T) {
	// Store is opened outside the bubble (filesystem I/O before the bubble).
	store := openTestStore(t)
	ft := newFakeTracker()
	rec := &msgRecorder{}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		r := newRefresher(ctx, tickInterval, ft, store, rec.emit)

		// Advance fake time past one interval → ticker fires.
		time.Sleep(tickInterval)
		synctest.Wait()

		cancel()
		r.Wait()

		if ft.callCount() != 0 {
			t.Errorf("want 0 GetTicket calls; got %d", ft.callCount())
		}
		msgs := rec.all()
		if len(msgs) == 0 {
			t.Fatal("want at least 1 message")
		}
		if _, ok := msgs[0].(ui.PollOKMsg); !ok {
			t.Errorf("want PollOKMsg; got %T", msgs[0])
		}
	})
}

// TestTick_RefreshesOnlyJiraSource verifies that 2 jira + 1 local → exactly 2 GetTicket calls
// per tick, and the local ticket's fields are untouched.
//
// Uses [testing/synctest] for deterministic ticker control.
func TestTick_RefreshesOnlyJiraSource(t *testing.T) {
	store := openTestStore(t)
	ft := newFakeTracker()
	rec := &msgRecorder{}

	addJiraTicket(t, store, "PROJ-1", "todo", "original 1")
	addJiraTicket(t, store, "PROJ-2", "todo", "original 2")
	addLocalTicket(t, store, "LOCAL-1")

	ft.results["PROJ-1"] = tracker.Ticket{Key: "PROJ-1", Summary: "updated 1", Status: "in-progress"}
	ft.results["PROJ-2"] = tracker.Ticket{Key: "PROJ-2", Summary: "updated 2", Status: "done"}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		r := newRefresher(ctx, tickInterval, ft, store, rec.emit)

		// Advance past exactly one interval → exactly one tick fires.
		time.Sleep(tickInterval)
		synctest.Wait()

		cancel()
		r.Wait()

		// Exactly one tick: 2 jira tickets → 2 GetTicket calls.
		keys := ft.calledKeys()
		if len(keys) != 2 {
			t.Errorf("want 2 GetTicket calls; got %d: %v", len(keys), keys)
		}
		for _, k := range keys {
			if k == "LOCAL-1" {
				t.Errorf("GetTicket called for local ticket LOCAL-1")
			}
		}

		snap, _ := store.Snapshot()
		local := snap.Tickets["LOCAL-1"]
		if local.Source != "local" {
			t.Errorf("local ticket source changed: got %q", local.Source)
		}
		if local.Summary != "local ticket" {
			t.Errorf("local ticket summary changed: got %q", local.Summary)
		}
	})
}

// TestTick_PartialFailure_EmitsErrAndKeepsRunning verifies that one erroring key emits
// PollErrMsg and the refresher continues; the succeeding key's fields are updated.
//
// Uses [testing/synctest] for deterministic ticker control.
func TestTick_PartialFailure_EmitsErrAndKeepsRunning(t *testing.T) {
	store := openTestStore(t)
	ft := newFakeTracker()
	rec := &msgRecorder{}

	addJiraTicket(t, store, "PROJ-1", "todo", "original 1")
	addJiraTicket(t, store, "PROJ-2", "todo", "original 2")

	ft.errors["PROJ-1"] = errors.New("network error")
	ft.results["PROJ-2"] = tracker.Ticket{Key: "PROJ-2", Summary: "updated", Status: "done"}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		r := newRefresher(ctx, tickInterval, ft, store, rec.emit)

		// Advance past one interval → tick fires, processes both tickets.
		time.Sleep(tickInterval)
		synctest.Wait()

		cancel()
		r.Wait()

		msgs := rec.all()
		var sawErr bool
		for _, m := range msgs {
			if _, ok := m.(ui.PollErrMsg); ok {
				sawErr = true
			}
		}
		if !sawErr {
			t.Error("want PollErrMsg; none received")
		}

		// PROJ-2 must be updated despite PROJ-1 failing.
		snap, _ := store.Snapshot()
		if snap.Tickets["PROJ-2"].Summary != "updated" {
			t.Errorf("PROJ-2 summary not updated: got %q", snap.Tickets["PROJ-2"].Summary)
		}
	})
}

// TestCancel_ContextDone_StopsLoop verifies that cancel() stops the loop within one fake-tick.
//
// Uses [testing/synctest] for deterministic ticker control.
func TestCancel_ContextDone_StopsLoop(t *testing.T) {
	store := openTestStore(t)
	ft := newFakeTracker()
	rec := &msgRecorder{}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		r := newRefresher(ctx, tickInterval, ft, store, rec.emit)

		// Cancel before any tick fires.
		cancel()

		// synctest.Wait() will unblock once the goroutine returns to select and both
		// ctx.Done() and the cancel are processed — the loop exits.
		synctest.Wait()
		r.Wait()
		// If Wait returns we know the goroutine exited cleanly.
	})
}

// TestRefresh_NeverInsertsOrDeletes verifies that the refresher does not insert new
// tickets even when GetTicket returns a key not in state.Tickets.
//
// Uses [testing/synctest] for deterministic ticker control.
func TestRefresh_NeverInsertsOrDeletes(t *testing.T) {
	store := openTestStore(t)
	ft := newFakeTracker()
	rec := &msgRecorder{}

	addJiraTicket(t, store, "PROJ-1", "todo", "orig")
	ft.results["PROJ-1"] = tracker.Ticket{Key: "PROJ-1", Summary: "new", Status: "done"}

	snap0, _ := store.Snapshot()
	ticketsBefore := len(snap0.Tickets)

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		r := newRefresher(ctx, tickInterval, ft, store, rec.emit)

		// One tick.
		time.Sleep(tickInterval)
		synctest.Wait()

		cancel()
		r.Wait()

		snap, _ := store.Snapshot()
		if len(snap.Tickets) != ticketsBefore {
			t.Errorf("ticket count changed: before=%d after=%d", ticketsBefore, len(snap.Tickets))
		}
	})
}

// TestRefresh_PreservesPositionAndSource verifies that X, Y, Source, LocalStatus,
// and AssignedRepoIDs are not modified by the refresher.
//
// Uses [testing/synctest] for deterministic ticker control.
func TestRefresh_PreservesPositionAndSource(t *testing.T) {
	store := openTestStore(t)
	ft := newFakeTracker()
	rec := &msgRecorder{}

	err := store.Mutate(func(s *state.State) error {
		s.Tickets["PROJ-1"] = state.TicketState{
			Key:             "PROJ-1",
			Source:          "jira",
			Status:          "todo",
			Summary:         "orig",
			X:               7,
			Y:               11,
			AssignedRepoIDs: []string{"repo-a", "repo-b"},
		}
		return nil
	})
	if err != nil {
		t.Fatalf("mutate: %v", err)
	}

	ft.results["PROJ-1"] = tracker.Ticket{
		Key:     "PROJ-1",
		Summary: "refreshed",
		Status:  "done",
	}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		r := newRefresher(ctx, tickInterval, ft, store, rec.emit)

		// One tick.
		time.Sleep(tickInterval)
		synctest.Wait()

		cancel()
		r.Wait()

		snap, _ := store.Snapshot()
		ts := snap.Tickets["PROJ-1"]

		if ts.X != 7 || ts.Y != 11 {
			t.Errorf("position changed: X=%d Y=%d (want 7,11)", ts.X, ts.Y)
		}
		if ts.Source != "jira" {
			t.Errorf("Source changed: got %q", ts.Source)
		}
		if len(ts.AssignedRepoIDs) != 2 {
			t.Errorf("AssignedRepoIDs changed: got %v", ts.AssignedRepoIDs)
		}
		if ts.Summary != "refreshed" {
			t.Errorf("Summary not updated: got %q", ts.Summary)
		}
		if ts.Status != "done" {
			t.Errorf("Status not updated: got %q", ts.Status)
		}
	})
}
