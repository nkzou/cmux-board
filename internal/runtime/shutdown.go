package runtime

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	gosync "sync"
	"syscall"
	"time"

	isync "github.com/nkzou/cmux-board/internal/sync"
	"github.com/nkzou/cmux-board/internal/state"
)

// SHUTDOWN INVARIANT: cmux-board NEVER calls any of the following on shutdown:
//   - claude stop <short_id>
//   - claude rm <short_id>
//   - cmux close-workspace ...
//   - cmux workspace-action --action close-* ...
//   - cmux kill ...
//
// Activations are ADDITIVE-ONLY. The user or cmux-board doctor handles cleanup.
// F-NEW invariant; verified by T-084 grep arm 1.

const shutdownDrainTimeout = 5 * time.Second

// pollerWaiter is the interface the Coordinator uses to drain the poller.
// Using an interface here enables test injection without a real *isync.Poller.
type pollerWaiter interface {
	Wait(timeout time.Duration) bool
}

// storeFlushable is the interface the Coordinator uses to flush state.
type storeFlushable interface {
	Flush() error
}

// Coordinator manages graceful shutdown of the dock process.
type Coordinator struct {
	cancel       context.CancelFunc
	poller       pollerWaiter
	store        storeFlushable
	logger       *slog.Logger
	exitFn       func(int)     // defaults to os.Exit; overridden in tests
	drainTimeout time.Duration // defaults to shutdownDrainTimeout; overridden in tests
}

// NewCoordinator creates a Coordinator. Call Run() to install signal handlers.
func NewCoordinator(cancel context.CancelFunc, poller *isync.Poller, store *state.Store, logger *slog.Logger) *Coordinator {
	return &Coordinator{
		cancel:       cancel,
		poller:       poller,
		store:        store,
		logger:       logger,
		exitFn:       os.Exit,
		drainTimeout: shutdownDrainTimeout,
	}
}

// newCoordinatorFromInterfaces constructs a Coordinator from interfaces for testing.
func newCoordinatorFromInterfaces(cancel context.CancelFunc, poller pollerWaiter, store storeFlushable, logger *slog.Logger, exitFn func(int)) *Coordinator {
	return &Coordinator{
		cancel:       cancel,
		poller:       poller,
		store:        store,
		logger:       logger,
		exitFn:       exitFn,
		drainTimeout: shutdownDrainTimeout,
	}
}

// Run installs SIGINT + SIGTERM handlers and blocks until a signal is received.
// When a signal arrives it:
//  1. Logs "shutdown: received <signal>; draining..." at Info level.
//  2. Calls cancel() to propagate context cancellation to all goroutines.
//  3. Calls poller.Wait() with a 5s deadline to drain in-flight poll cycle.
//  4. Calls store.Flush() to persist the final state.json atomically.
//  5. Logs "shutdown: complete" at Info level.
//  6. Returns. The caller (dock_cmd.go RunE) then returns nil → cobra exits 0.
func (c *Coordinator) Run(ctx context.Context) {
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case sig := <-sigCh:
		c.logger.Info("shutdown: received signal; draining...", "signal", sig.String())
	case <-ctx.Done():
		// Context cancelled externally (e.g. BubbleTea q-press).
		c.logger.Info("shutdown: context done; draining...")
	}

	// Step 2: propagate cancellation.
	c.cancel()

	// drainDone is closed when poller drain + flush complete.
	// The second-signal goroutine waits on it to exit cleanly, and we join it
	// with a WaitGroup before Run returns to prevent the goroutine from
	// accessing the logger after the test has finished.
	drainDone := make(chan struct{})
	var wg gosync.WaitGroup
	wg.Add(1)

	// Second-signal handler: if user presses Ctrl-C again while draining, force exit.
	go func() {
		defer wg.Done()
		select {
		case <-sigCh:
			c.logger.Info("shutdown: second signal; forcing exit")
			c.exitFn(1)
		case <-drainDone:
			// Drain completed before second signal — goroutine exits cleanly.
		}
	}()

	// Step 3: drain poller.
	if ok := c.poller.Wait(c.drainTimeout); !ok {
		c.logger.Warn("shutdown: poller drain timeout; continuing")
	}

	// Step 4: flush state.
	if err := c.store.Flush(); err != nil {
		c.logger.Error("shutdown: state flush failed", "err", err)
		// Best-effort: process is exiting; do not abort.
	}

	close(drainDone)
	wg.Wait() // ensure second-signal goroutine has exited before Run returns
	c.logger.Info("shutdown: complete")
}
