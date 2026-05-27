package sync

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kevin-zou/cmux-board/internal/config"
	"github.com/kevin-zou/cmux-board/internal/state"
	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// Poller wraps the background poll goroutine and provides a Wait method for
// clean shutdown. Construct via NewPoller; do not create directly.
type Poller struct {
	done chan struct{}
}

// NewPoller starts the background poll goroutine and returns a Poller that
// can be used to wait for it to exit. The goroutine stops when ctx is cancelled.
//
// This is the preferred entry point for production code; the package-level
// Start function is kept for backward compatibility.
func NewPoller(
	ctx context.Context,
	cfg *config.Config,
	tr tracker.IssueTracker,
	store *state.Store,
	emitFn func(tea.Msg),
) *Poller {
	p := &Poller{done: make(chan struct{})}
	go func() {
		defer close(p.done)
		// Delegate to the existing polling logic.  We wrap in a goroutine that
		// closes done when Start's goroutine exits.  Because Start does not
		// currently expose its goroutine's done signal, we reproduce its
		// loop inline here to get proper lifetime control.
		runPollLoop(ctx, cfg, tr, store, emitFn)
	}()
	return p
}

// Wait blocks until the poll goroutine exits or timeout elapses.
// Returns true if the goroutine exited cleanly; false on timeout.
func (p *Poller) Wait(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-p.done:
		return true
	case <-timer.C:
		return false
	}
}

// runPollLoop is the inner poll loop shared by NewPoller.
// It mirrors Start but runs synchronously within the caller's goroutine.
func runPollLoop(
	ctx context.Context,
	cfg *config.Config,
	tr tracker.IssueTracker,
	store *state.Store,
	emitFn func(tea.Msg),
) {
	interval := time.Duration(cfg.PollIntervalSeconds) * time.Second
	if interval < minPollInterval {
		interval = minPollInterval
	}

	var bo Backoff
	var wg sync.WaitGroup

	runOnePoll := func() time.Duration {
		wg.Add(1)
		defer wg.Done()
		if err := poll(ctx, cfg, tr, store); err != nil {
			kind := classifyPollError(err)
			emitFn(PollErrMsg{Kind: kind, At: time.Now()})
			return bo.Next()
		}
		bo.Reset()
		emitFn(PollOKMsg{At: time.Now()})
		return interval
	}

	nextWait := runOnePoll()
	timer := time.NewTimer(nextWait)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-timer.C:
			nextWait = runOnePoll()
			timer.Reset(nextWait)
		}
	}
}
