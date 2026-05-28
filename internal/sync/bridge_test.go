package sync

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// TestBridgePushOKMsg: PushIntent on channel → success → PushOKMsg on ResultCh;
// state.Store last_known_status is updated.
func TestBridgePushOKMsg(t *testing.T) {
	store := seedStore(t, "To Do")
	tr := &mockTracker{}

	b := NewBridge()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.RunPushWorker(ctx, store, tr)

	b.PushIntentCh <- PushIntent{
		TicketID:     "PROJ-42",
		TargetStatus: "In Progress",
	}

	msg := waitMsg(t, b.ResultCh, 2*time.Second)
	ok, isOK := msg.(PushOKMsg)
	if !isOK {
		t.Fatalf("expected PushOKMsg, got %T", msg)
	}
	if ok.TicketID != "PROJ-42" {
		t.Errorf("TicketID = %q, want %q", ok.TicketID, "PROJ-42")
	}
	if ok.NewStatus != "In Progress" {
		t.Errorf("NewStatus = %q, want %q", ok.NewStatus, "In Progress")
	}

	snap, _ := store.Snapshot()
	if snap.Tickets["PROJ-42"].LastKnownStatus != "In Progress" {
		t.Errorf("LastKnownStatus = %q, want %q",
			snap.Tickets["PROJ-42"].LastKnownStatus, "In Progress")
	}
}

// TestBridgePushConflictMsg: tracker returns ErrConflict → PushConflictMsg;
// state.Store is NOT mutated (last_known_status stays at original value).
func TestBridgePushConflictMsg(t *testing.T) {
	store := seedStore(t, "To Do")
	tr := &mockTracker{transitionErr: tracker.ErrConflict}

	b := NewBridge()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.RunPushWorker(ctx, store, tr)

	b.PushIntentCh <- PushIntent{
		TicketID:     "PROJ-42",
		TargetStatus: "In Progress",
	}

	msg := waitMsg(t, b.ResultCh, 2*time.Second)
	_, isConflict := msg.(PushConflictMsg)
	if !isConflict {
		t.Fatalf("expected PushConflictMsg, got %T", msg)
	}

	// No state mutation should have occurred.
	snap, _ := store.Snapshot()
	if snap.Tickets["PROJ-42"].LastKnownStatus != "To Do" {
		t.Errorf("LastKnownStatus mutated on conflict: got %q, want %q",
			snap.Tickets["PROJ-42"].LastKnownStatus, "To Do")
	}
}

// TestBridgePoller: poller with a fake clock emits PollOKMsg into ResultCh via EmitFn.
func TestBridgePoller(t *testing.T) {
	store := seedStore(t, "To Do")

	// Tracker returns one ticket on ListTickets.
	tr := &bridgePollMockTracker{
		tickets: []tracker.Ticket{{ID: "PROJ-42", Key: "PROJ-42", Summary: "test", Status: "To Do"}},
	}

	b := NewBridge()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &config.Config{
		BoardID:             "BOARD-1",
		PollIntervalSeconds: 600, // large; only want the immediate first poll
	}

	// Start poller — it fires one immediate poll, emitting PollOKMsg.
	_ = NewPoller(ctx, cfg, tr, store, b.EmitFn())

	msg := waitMsg(t, b.ResultCh, 3*time.Second)
	_, isOK := msg.(PollOKMsg)
	if !isOK {
		t.Fatalf("expected PollOKMsg, got %T", msg)
	}
}

// TestBridgeContextCancel: cancelling the context causes RunPushWorker to exit within 100ms.
func TestBridgeContextCancel(t *testing.T) {
	store := seedStore(t, "To Do")
	tr := &mockTracker{}

	b := NewBridge()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		b.RunPushWorker(ctx, store, tr)
	}()

	cancel()

	select {
	case <-done:
		// Clean exit.
	case <-time.After(100 * time.Millisecond):
		t.Fatal("RunPushWorker did not exit within 100ms of context cancel")
	}
}

// waitMsg blocks until a message arrives on ch or deadline is exceeded.
func waitMsg(t *testing.T, ch <-chan tea.Msg, d time.Duration) tea.Msg {
	t.Helper()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(d):
		t.Fatalf("timed out waiting for message after %v", d)
		return nil
	}
}

// bridgePollMockTracker returns a fixed ticket list and a minimal board on GetBoard.
type bridgePollMockTracker struct {
	tickets []tracker.Ticket
}

func (m *bridgePollMockTracker) WhoAmI(_ context.Context) (tracker.UserIdentity, error) {
	return tracker.UserIdentity{}, nil
}
func (m *bridgePollMockTracker) ListBoards(_ context.Context) ([]tracker.BoardSummary, error) {
	return nil, nil
}
func (m *bridgePollMockTracker) GetBoard(_ context.Context, _ string) (tracker.Board, error) {
	return tracker.Board{
		ID:   "BOARD-1",
		Name: "Test Board",
		Columns: []tracker.Column{
			{ID: "col-1", Name: "To Do", StatusIDs: []string{"To Do"}},
		},
	}, nil
}
func (m *bridgePollMockTracker) ListTickets(_ context.Context, _ string, _ *time.Time) ([]tracker.Ticket, error) {
	return m.tickets, nil
}
func (m *bridgePollMockTracker) TransitionStatus(_ context.Context, _, _, _ string) error {
	return nil
}
func (m *bridgePollMockTracker) Capabilities() tracker.Capabilities {
	return tracker.Capabilities{}
}
