package refresh

import (
	"context"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
	"github.com/nkzou/cmux-board/internal/ui"
)

// minRefreshInterval is the hard floor for refresh intervals (10s).
const minRefreshInterval = 10 * time.Second

// Config holds the configuration for a Refresher.
type Config struct {
	// Interval is the time between refresh ticks.
	// Values below 10s are clamped up to 10s.
	Interval time.Duration
}

// Refresher reads existing jira-sourced tickets from the store, fetches their
// current state from the tracker, and writes back ONLY data fields.
// It never inserts or deletes tickets — membership is owned by the full poller.
type Refresher struct {
	done chan struct{}
}

// NewRefresher creates and starts a Refresher goroutine. The goroutine runs until
// ctx is cancelled. Call Wait() to block until it has fully exited.
// Intervals below minRefreshInterval are clamped up with a warning.
func NewRefresher(
	ctx context.Context,
	cfg Config,
	tr tracker.IssueTracker,
	store *state.Store,
	emit func(tea.Msg),
) *Refresher {
	interval := cfg.Interval
	if interval < minRefreshInterval {
		slog.Warn("refresh interval < 10s; clamped to 10s", "requested", interval)
		interval = minRefreshInterval
	}
	return newRefresher(ctx, interval, tr, store, emit)
}

// newRefresher is the internal constructor that takes a pre-validated interval.
// Tests use this to pass sub-10s intervals without triggering the production clamp.
func newRefresher(
	ctx context.Context,
	interval time.Duration,
	tr tracker.IssueTracker,
	store *state.Store,
	emit func(tea.Msg),
) *Refresher {
	r := &Refresher{
		done: make(chan struct{}),
	}
	go r.runLoop(ctx, interval, tr, store, emit)
	return r
}

// Wait blocks until the refresh goroutine has exited.
// Safe to call after ctx is cancelled.
func (r *Refresher) Wait() {
	<-r.done
}

// runLoop is the main goroutine. It ticks every interval and calls refresh.
func (r *Refresher) runLoop(
	ctx context.Context,
	interval time.Duration,
	tr tracker.IssueTracker,
	store *state.Store,
	emit func(tea.Msg),
) {
	defer close(r.done)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tick(ctx, tr, store, emit)
		}
	}
}

// tick executes one refresh cycle.
func (r *Refresher) tick(
	ctx context.Context,
	tr tracker.IssueTracker,
	store *state.Store,
	emit func(tea.Msg),
) {
	snap, _ := store.Snapshot()

	var failed bool
	for k, ts := range snap.Tickets {
		if ts.Source != "jira" {
			continue
		}

		t, err := tr.GetTicket(ctx, k)
		if err != nil {
			errClass := classifyError(err)
			slog.Error("refresh: GetTicket failed", "key", k, "error_class", errClass)
			emit(ui.PollErrMsg{
				Code: errClass,
				When: time.Now(),
			})
			failed = true
			continue
		}

		mutErr := store.Mutate(func(s *state.State) error {
			cur, ok := s.Tickets[k]
			if !ok {
				// Ticket removed between snapshot and mutate — never insert.
				return nil
			}
			now := time.Now().UTC()
			cur.Status = t.Status
			cur.Summary = t.Summary
			cur.Priority = t.Priority
			cur.AssigneeID = t.AssigneeID
			cur.URL = t.URL
			cur.Labels = t.Labels
			cur.UpdatedAt = &now
			// X, Y, Source, LocalStatus, AssignedRepoIDs are intentionally not touched.
			s.Tickets[k] = cur
			return nil
		})
		if mutErr != nil {
			slog.Error("refresh: mutate failed", "key", k, "err", mutErr)
		}
	}

	if !failed {
		emit(ui.PollOKMsg{})
	}
}

// classifyError maps an error to a short string for the PollErrMsg.Code field
// and for structured log fields. Only the class is logged — never the error message,
// which may contain ticket content.
func classifyError(err error) string {
	if tracker.IsAuthError(err) {
		return "auth"
	}
	return "transient"
}
