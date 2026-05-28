package sync

import (
	"context"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// minPollInterval is the hard floor for poll intervals.
// Configurations below this value are clamped with a one-time startup warning.
const minPollInterval = 10 * time.Second

// classifyPollError maps a tracker error to the appropriate PollErrorKind.
// 401 auth failures → PollErrorAuth; everything else → PollErrorTransient.
func classifyPollError(err error) PollErrorKind {
	if tracker.IsAuthError(err) {
		return PollErrorAuth
	}
	return PollErrorTransient
}

// Start launches the background poll goroutine and returns immediately.
// The goroutine runs until ctx is cancelled.
//
// On each interval it calls tr.ListTickets + tr.GetBoard, merges results into
// the store via MergePulledTickets, and emits a PollOKMsg or PollErrMsg via emitFn.
// Backoff is applied on consecutive errors; Reset is called on first success.
func Start(
	ctx    context.Context,
	cfg    *config.Config,
	tr     tracker.IssueTracker,
	store  *state.Store,
	emitFn func(tea.Msg),
) {
	interval := time.Duration(cfg.PollIntervalSeconds) * time.Second

	// Clamp to minimum poll interval with a one-time startup warning.
	if interval < minPollInterval {
		original := interval
		interval = minPollInterval
		slog.Warn("poll_interval_seconds < 10; clamped to 10s",
			"requested", original)
	}

	go func() {
		var bo Backoff

		// Execute an immediate first poll before the ticker fires so the UI
		// has data on startup without waiting a full interval.
		runOnePoll := func() (wait time.Duration) {
			if err := poll(ctx, cfg, tr, store); err != nil {
				kind := classifyPollError(err)
				emitFn(PollErrMsg{Kind: kind, At: time.Now()})
				return bo.Next() // backoff on error
			}
			bo.Reset()
			emitFn(PollOKMsg{At: time.Now()})
			return interval // normal interval on success
		}

		// First poll immediately (no initial wait).
		nextWait := runOnePoll()

		// Subsequent polls on a dynamic timer driven by the last result.
		timer := time.NewTimer(nextWait)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				nextWait = runOnePoll()
				timer.Reset(nextWait)
			}
		}
	}()
}

// poll executes one poll iteration: ListTickets + GetBoard + MergePulledTickets.
// Returns the first error encountered; on success returns nil.
func poll(
	ctx   context.Context,
	cfg   *config.Config,
	tr    tracker.IssueTracker,
	store *state.Store,
) error {
	tickets, err := tr.ListTickets(ctx, cfg.BoardID, nil)
	if err != nil {
		return err
	}

	board, err := tr.GetBoard(ctx, cfg.BoardID)
	if err != nil {
		return err
	}

	// All state mutations go through store.Mutate — no direct assignment.
	// NOTE: This poller is deprecated and will be deleted in M-6/T-602.
	return store.Mutate(func(s *state.State) error {
		MergePulledTickets(s, tickets)
		_ = board // board layout not stored in State (schema v3; UI rewritten in M-4)
		return nil
	})
}

