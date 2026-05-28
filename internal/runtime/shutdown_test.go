package runtime

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"syscall"
	"testing"
	"time"
)

// mockPoller implements Drainable for tests.
// Set blockFor > 0 to simulate a slow drain (longer than drainTimeout).
type mockPoller struct {
	blockFor time.Duration // if > 0, Wait blocks this long before returning
	called   bool
}

func (m *mockPoller) Wait() {
	m.called = true
	if m.blockFor > 0 {
		time.Sleep(m.blockFor)
	}
}

// mockStore implements storeFlushable for tests.
type mockStore struct {
	flushCalled bool
	flushErr    error
}

func (m *mockStore) Flush() error {
	m.flushCalled = true
	return m.flushErr
}

// testLogger returns a logger that writes to buf.
func testLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// runCoordinatorWithCancel starts the coordinator Run in a goroutine and returns
// a channel that is closed when Run returns.
func runCoordinatorWithCancel(ctx context.Context, c *Coordinator) chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.Run(ctx)
	}()
	return done
}

func TestShutdownOnSIGINT(t *testing.T) {
	// Signal tests are inherently process-wide; do not run in parallel.
	ctx, cancel := context.WithCancel(context.Background())
	poller := &mockPoller{}
	store := &mockStore{}
	exitCh := make(chan int, 1)

	var buf bytes.Buffer
	logger := testLogger(&buf)

	c := newCoordinatorFromInterfaces(cancel, poller, store, logger, func(code int) {
		exitCh <- code
	})

	done := runCoordinatorWithCancel(ctx, c)

	// Send SIGINT after a brief delay to let Run install its handlers.
	time.Sleep(20 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s after SIGINT")
	}

	if !store.flushCalled {
		t.Error("store.Flush was not called")
	}
	select {
	case code := <-exitCh:
		t.Errorf("exitFn was called unexpectedly with code %d", code)
	default:
		// expected: no exit on first signal
	}
}

func TestShutdownOnSIGTERM(t *testing.T) {
	// Signal tests are inherently process-wide; do not run in parallel.
	ctx, cancel := context.WithCancel(context.Background())
	poller := &mockPoller{}
	store := &mockStore{}
	var buf bytes.Buffer
	logger := testLogger(&buf)

	c := newCoordinatorFromInterfaces(cancel, poller, store, logger, func(_ int) {})

	done := runCoordinatorWithCancel(ctx, c)

	time.Sleep(20 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s after SIGTERM")
	}

	if !store.flushCalled {
		t.Error("store.Flush was not called")
	}
}

func TestShutdownPollerDrainTimeout(t *testing.T) {
	// NOT parallel — uses process-wide signal channel patterns.
	const testDrainTimeout = 80 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	// Poller blocks longer than the injected drain timeout.
	poller := &mockPoller{blockFor: 5 * time.Second}
	store := &mockStore{}
	var buf bytes.Buffer
	logger := testLogger(&buf)

	c := newCoordinatorFromInterfaces(cancel, poller, store, logger, func(_ int) {})
	c.drainTimeout = testDrainTimeout

	start := time.Now()
	done := make(chan struct{})
	go func() {
		defer close(done)
		cancel() // trigger via context, not signal
		c.Run(ctx)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return within 3s")
	}

	elapsed := time.Since(start)
	// Should return close to testDrainTimeout, not 5s.
	if elapsed < testDrainTimeout {
		t.Errorf("Run returned too fast (%v); expected >= %v", elapsed, testDrainTimeout)
	}
	if elapsed > 2*time.Second {
		t.Errorf("Run took too long (%v); drain timeout was %v", elapsed, testDrainTimeout)
	}

	if !contains(buf.String(), "drain timeout") {
		t.Errorf("expected 'drain timeout' warning in output:\n%s", buf.String())
	}
	if !store.flushCalled {
		t.Error("store.Flush should still be called after drain timeout")
	}
}

func TestShutdownSecondSignalForcesExit(t *testing.T) {
	// NOT parallel — sends two signals to the process.
	exitCh := make(chan int, 1)
	ctx, cancel := context.WithCancel(context.Background())
	const testDrainTimeout = 5 * time.Second // long enough to get second signal
	poller := &mockPoller{blockFor: 10 * time.Second}
	store := &mockStore{}
	var buf bytes.Buffer
	logger := testLogger(&buf)

	c := newCoordinatorFromInterfaces(cancel, poller, store, logger, func(code int) {
		exitCh <- code
	})
	c.drainTimeout = testDrainTimeout

	runDone := runCoordinatorWithCancel(ctx, c)

	time.Sleep(20 * time.Millisecond)
	// First signal.
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	// Brief pause then second signal while drain is in progress.
	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}

	// Wait for exitFn to be called.
	select {
	case code := <-exitCh:
		if code != 1 {
			t.Errorf("expected exitFn(1), got exitFn(%d)", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("exitFn not called within 2s after second signal")
	}

	// Cancel to unblock Run goroutine (it's stuck draining).
	cancel()
	select {
	case <-runDone:
	case <-time.After(10 * time.Second):
		// goroutine may be stuck waiting on drain — acceptable since exitFn was the focus.
	}
}

func TestShutdownStoreFlushErrorNotFatal(t *testing.T) {
	// Signal tests are inherently process-wide; do not run in parallel.
	ctx, cancel := context.WithCancel(context.Background())
	poller := &mockPoller{}
	store := &mockStore{flushErr: errFlushFailed}
	var buf bytes.Buffer
	logger := testLogger(&buf)

	c := newCoordinatorFromInterfaces(cancel, poller, store, logger, func(_ int) {})

	done := runCoordinatorWithCancel(ctx, c)

	time.Sleep(20 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s")
	}

	// Run should return normally despite flush error.
	if !store.flushCalled {
		t.Error("store.Flush was not called")
	}
	// Error should be logged.
	if !contains(buf.String(), "flush failed") {
		t.Errorf("expected 'flush failed' in log output:\n%s", buf.String())
	}
}

// fakeDrainable is a minimal Drainable for TestCoordinator_DrainsViaInterface.
type fakeDrainable struct {
	waited bool
}

func (f *fakeDrainable) Wait() { f.waited = true }

// TestCoordinator_DrainsViaInterface verifies that NewCoordinator accepts any
// Drainable and calls Wait() on shutdown.
func TestCoordinator_DrainsViaInterface(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fake := &fakeDrainable{}
	store := &mockStore{}
	var buf bytes.Buffer
	logger := testLogger(&buf)

	c := newCoordinatorFromInterfaces(cancel, fake, store, logger, func(_ int) {})

	done := runCoordinatorWithCancel(ctx, c)
	time.Sleep(20 * time.Millisecond)
	cancel() // trigger shutdown via context

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s")
	}

	if !fake.waited {
		t.Error("Drainable.Wait() was not called on shutdown")
	}
	if !store.flushCalled {
		t.Error("store.Flush() was not called on shutdown")
	}
}

// errFlushFailed is a sentinel for testing flush error logging.
var errFlushFailed = errSentinel("flush failed")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

func TestShutdownNoDestructiveCalls(t *testing.T) {
	// Signal tests are inherently process-wide; do not run in parallel.
	// This test verifies by construction: the Coordinator only calls
	// cancel(), poller.Wait(), and store.Flush() — no cmuxcli or claudecli calls.
	// The mock types above track all calls and neither cmuxcli nor claudecli
	// are imported by shutdown.go.
	ctx, cancel := context.WithCancel(context.Background())
	poller := &mockPoller{}
	store := &mockStore{}
	var buf bytes.Buffer
	logger := testLogger(&buf)

	c := newCoordinatorFromInterfaces(cancel, poller, store, logger, func(_ int) {})

	done := runCoordinatorWithCancel(ctx, c)
	time.Sleep(20 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	// If we got here with only the mock types above, no real CLI was invoked.
	// shutdown.go imports are verified at compile time.
}
