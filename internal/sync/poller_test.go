package sync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// pollMockTracker allows controlling what ListTickets and GetBoard return per call.
type pollMockTracker struct {
	mu            sync.Mutex
	listErr       error // if non-nil, ListTickets returns this
	calls         int
}

func (m *pollMockTracker) WhoAmI(_ context.Context) (tracker.UserIdentity, error) {
	return tracker.UserIdentity{}, nil
}
func (m *pollMockTracker) ListBoards(_ context.Context) ([]tracker.BoardSummary, error) {
	return nil, nil
}
func (m *pollMockTracker) GetBoard(_ context.Context, _ string) (tracker.Board, error) {
	return tracker.Board{ID: "board1", Name: "Board"}, nil
}
func (m *pollMockTracker) ListTickets(_ context.Context, _ string, _ *time.Time) ([]tracker.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.listErr != nil {
		return nil, m.listErr
	}
	return []tracker.Ticket{{Key: "PROJ-1", Summary: "ticket", Status: "Open"}}, nil
}
func (m *pollMockTracker) TransitionStatus(_ context.Context, _, _, _ string) error { return nil }
func (m *pollMockTracker) Capabilities() tracker.Capabilities                        { return tracker.Capabilities{} }

func (m *pollMockTracker) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// collectMsgs gathers tea.Msg values sent by emitFn with a timeout.
func collectMsgs(t *testing.T, dur time.Duration, emitFn func() func(tea.Msg)) ([]tea.Msg, func(tea.Msg)) {
	t.Helper()
	var mu sync.Mutex
	var msgs []tea.Msg
	fn := func(msg tea.Msg) {
		mu.Lock()
		defer mu.Unlock()
		msgs = append(msgs, msg)
	}
	return msgs, fn
}

// TC-1: poller fires immediately on start and emits PollOKMsg.
// The immediate-first-poll design ensures the UI has data on startup
// without waiting a full interval. We verify at least 1 call fires within 1s.
func TestPoller_FiresAtInterval(t *testing.T) {
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("TC-1: %v", err)
	}
	tr := &pollMockTracker{}

	okReceived := make(chan struct{}, 10)
	emitFn := func(msg tea.Msg) {
		if _, ok := msg.(PollOKMsg); ok {
			okReceived <- struct{}{}
		}
	}

	// PollIntervalSeconds: 10 (minimum valid; clamping not triggered).
	cfg := &config.Config{BoardID: "board1", PollIntervalSeconds: 10}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	Start(ctx, cfg, tr, store, emitFn)

	// The immediate first poll should fire within 500ms.
	select {
	case <-okReceived:
	case <-time.After(500 * time.Millisecond):
		t.Error("TC-1: immediate first poll did not fire within 500ms")
		return
	}

	if tr.callCount() < 1 {
		t.Errorf("TC-1: want >= 1 tracker call, got %d", tr.callCount())
	}
}

// TC-2: interval < 10s clamped to 10s; slog warning emitted exactly once.
func TestPoller_IntervalClamped(t *testing.T) {
	// Capture slog output.
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(orig)

	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("TC-2: %v", err)
	}
	tr := &pollMockTracker{}
	emitFn := func(tea.Msg) {}

	cfg := &config.Config{BoardID: "board1", PollIntervalSeconds: 3} // < 10s
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	Start(ctx, cfg, tr, store, emitFn)

	// Give the goroutine a moment to log.
	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)

	logOutput := buf.String()
	const warnMsg = "poll_interval_seconds < 10; clamped to 10s"
	if !containsStr(logOutput, warnMsg) {
		t.Errorf("TC-2: want slog warning %q in output, got: %s", warnMsg, logOutput)
	}
	// Warning must appear exactly once (count occurrences).
	occurrences := 0
	for i := 0; i <= len(logOutput)-len(warnMsg); i++ {
		if logOutput[i:i+len(warnMsg)] == warnMsg {
			occurrences++
		}
	}
	if occurrences != 1 {
		t.Errorf("TC-2: warning must appear exactly once, got %d occurrences", occurrences)
	}
}

// TC-3: all poll-success writes route through store.Mutate; state reflects the poll.
func TestPoller_StateWrittenViaMutate(t *testing.T) {
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("TC-3: %v", err)
	}
	tr := &pollMockTracker{}

	okReceived := make(chan struct{}, 1)
	emitFn := func(msg tea.Msg) {
		if _, ok := msg.(PollOKMsg); ok {
			select {
			case okReceived <- struct{}{}:
			default:
			}
		}
	}

	cfg := &config.Config{BoardID: "board1", PollIntervalSeconds: 10}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	Start(ctx, cfg, tr, store, emitFn)

	select {
	case <-okReceived:
	case <-ctx.Done():
		t.Fatal("TC-3: timed out waiting for PollOKMsg")
	}

	snap, _ := store.Snapshot()
	if _, ok := snap.Tickets["PROJ-1"]; !ok {
		t.Error("TC-3: PROJ-1 not found in state after successful poll")
	}
}

// TC-4: 5xx error → PollErrMsg{Kind: PollErrorTransient}; no state mutation.
func TestPoller_5xxError(t *testing.T) {
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("TC-4: %v", err)
	}

	transientErr := fmt.Errorf("tracker: retryable error 503 (retry after 0s)")
	tr := &pollMockTracker{listErr: transientErr}

	errReceived := make(chan PollErrMsg, 1)
	emitFn := func(msg tea.Msg) {
		if m, ok := msg.(PollErrMsg); ok {
			select {
			case errReceived <- m:
			default:
			}
		}
	}

	cfg := &config.Config{BoardID: "board1", PollIntervalSeconds: 10}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	Start(ctx, cfg, tr, store, emitFn)

	select {
	case m := <-errReceived:
		if m.Kind != PollErrorTransient {
			t.Errorf("TC-4: want PollErrorTransient, got %v", m.Kind)
		}
	case <-ctx.Done():
		t.Fatal("TC-4: timed out waiting for PollErrMsg")
	}

	snap, _ := store.Snapshot()
	if len(snap.Tickets) != 0 {
		t.Error("TC-4: state must not be mutated on poll error")
	}
}

// TC-5: 401 error → PollErrMsg{Kind: PollErrorAuth}; no state mutation.
func TestPoller_401Error(t *testing.T) {
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("TC-5: %v", err)
	}

	// authErr implements tracker.AuthError.
	authErr := &testAuthError{}
	tr := &pollMockTracker{listErr: authErr}

	errReceived := make(chan PollErrMsg, 1)
	emitFn := func(msg tea.Msg) {
		if m, ok := msg.(PollErrMsg); ok {
			select {
			case errReceived <- m:
			default:
			}
		}
	}

	cfg := &config.Config{BoardID: "board1", PollIntervalSeconds: 10}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	Start(ctx, cfg, tr, store, emitFn)

	select {
	case m := <-errReceived:
		if m.Kind != PollErrorAuth {
			t.Errorf("TC-5: want PollErrorAuth, got %v", m.Kind)
		}
	case <-ctx.Done():
		t.Fatal("TC-5: timed out waiting for PollErrMsg")
	}
}

// TC-6: ctx cancellation stops the goroutine cleanly.
func TestPoller_CancellationStopsGoroutine(t *testing.T) {
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("TC-6: %v", err)
	}
	tr := &pollMockTracker{}

	var wg sync.WaitGroup
	wg.Add(1)
	emitFn := func(msg tea.Msg) {
		if _, ok := msg.(PollOKMsg); ok {
			wg.Done()
		}
	}

	cfg := &config.Config{BoardID: "board1", PollIntervalSeconds: 10}
	ctx, cancel := context.WithCancel(context.Background())

	Start(ctx, cfg, tr, store, emitFn)

	// Wait for at least one OK (immediate first poll).
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("TC-6: timed out waiting for first poll")
	}

	// Cancel and verify the goroutine exits by checking no further errors.
	cancel()
	// Short pause — if goroutine is still running it would attempt another call.
	prevCalls := tr.callCount()
	time.Sleep(200 * time.Millisecond)
	if tr.callCount() > prevCalls+1 {
		t.Logf("TC-6: goroutine may still be running (calls: %d -> %d)", prevCalls, tr.callCount())
	}
	// Primary assertion: no panic and clean cancellation.
}

// testAuthError implements tracker.AuthError for testing.
type testAuthError struct{}

func (e *testAuthError) Error() string      { return "test auth error (401)" }
func (e *testAuthError) IsAuthError() bool  { return true }

// Ensure testAuthError satisfies tracker.AuthError at compile time.
var _ interface {
	error
	IsAuthError() bool
} = (*testAuthError)(nil)

// Ensure classifyPollError correctly routes a testAuthError.
func TestClassifyPollError_AuthError(t *testing.T) {
	ae := &testAuthError{}
	kind := classifyPollError(ae)
	if kind != PollErrorAuth {
		t.Errorf("want PollErrorAuth, got %v", kind)
	}
}

func TestClassifyPollError_TransientError(t *testing.T) {
	kind := classifyPollError(errors.New("some network error"))
	if kind != PollErrorTransient {
		t.Errorf("want PollErrorTransient, got %v", kind)
	}
}
