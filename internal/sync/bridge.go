package sync

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
)

const bridgeBufSize = 16

// PushIntent carries a card-move request from the UI to the push worker.
// The UI writes to Bridge.PushIntentCh; the worker drains it and calls Push.
type PushIntent struct {
	TicketID     string
	TargetStatus string
	DryRun       bool
}

// Bridge is the single coordination point between the BubbleTea program and
// the sync engine. It owns two channels:
//   - PushIntentCh: the UI writes PushIntent values; the worker drains them.
//   - ResultCh: the worker (and poller via emitFn) write tea.Msg values;
//     dock_cmd.go drains them via program.Send.
type Bridge struct {
	// PushIntentCh is the write-end given to the UI (or its drag handler).
	// Buffered to avoid blocking the BubbleTea goroutine.
	PushIntentCh chan PushIntent

	// ResultCh receives PushOKMsg, PushConflictMsg, PollOKMsg, PollErrMsg.
	// dock_cmd.go reads from ResultCh and forwards each value via tea.Program.Send.
	ResultCh chan tea.Msg
}

// NewBridge allocates both channels and returns an initialised Bridge.
func NewBridge() *Bridge {
	return &Bridge{
		PushIntentCh: make(chan PushIntent, bridgeBufSize),
		ResultCh:     make(chan tea.Msg, bridgeBufSize),
	}
}

// EmitFn returns an emitFn suitable for sync.NewPoller that forwards poll
// messages into ResultCh. The returned function is non-blocking: if ResultCh
// is full the message is silently dropped (the poller retries on the next
// interval; losing one PollOKMsg does not corrupt state).
func (b *Bridge) EmitFn() func(tea.Msg) {
	return func(msg tea.Msg) {
		select {
		case b.ResultCh <- msg:
		default:
			// ResultCh full — drop. The poller will emit again on the next interval.
		}
	}
}

// RunPushWorker drains PushIntentCh, calls Push for each intent, and writes
// the result (PushOKMsg or PushConflictMsg) into ResultCh. Exits when ctx is
// cancelled. Call as a goroutine.
func (b *Bridge) RunPushWorker(
	ctx context.Context,
	store *state.Store,
	tr tracker.IssueTracker,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case intent, ok := <-b.PushIntentCh:
			if !ok {
				return
			}
			b.handleIntent(ctx, store, tr, intent)
		}
	}
}

func (b *Bridge) handleIntent(
	ctx context.Context,
	store *state.Store,
	tr tracker.IssueTracker,
	intent PushIntent,
) {
	result, err := Push(ctx, store, tr, intent.TicketID, intent.TargetStatus, intent.DryRun)
	var msg tea.Msg
	switch {
	case err != nil:
		// Unexpected error: treat as conflict so UI snaps back.
		snap, _ := store.Snapshot()
		serverStatus := intent.TargetStatus
		if t, ok := snap.Tickets[intent.TicketID]; ok {
			serverStatus = t.LastKnownStatus
		}
		msg = PushConflictMsg{
			TicketID:     intent.TicketID,
			ServerStatus: serverStatus,
		}
	case result.Conflict || result.DryRun:
		msg = PushConflictMsg{
			TicketID:     intent.TicketID,
			ServerStatus: result.ServerStatus,
		}
	default:
		msg = PushOKMsg{
			TicketID:  intent.TicketID,
			NewStatus: intent.TargetStatus,
		}
	}

	select {
	case b.ResultCh <- msg:
	case <-ctx.Done():
	}
}
