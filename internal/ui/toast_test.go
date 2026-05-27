package ui

import (
	"errors"
	"testing"
	"time"

	"github.com/kevin-zou/cmux-board/internal/cmuxcli"
	"github.com/kevin-zou/cmux-board/internal/config"
	"github.com/kevin-zou/cmux-board/internal/git"
	"github.com/kevin-zou/cmux-board/internal/state"
	"github.com/kevin-zou/cmux-board/internal/tracker"
)

func makeTestModelForToast(t *testing.T) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	cfg := &config.Config{SchemaVersion: config.SchemaVersionCurrent}
	return NewModel(cfg, store)
}

// TestPushToast_AppendsEntry verifies that pushToast appends exactly one entry.
func TestPushToast_AppendsEntry(t *testing.T) {
	m := makeTestModelForToast(t)
	m, _ = m.pushToast("hello")
	if len(m.toasts) != 1 {
		t.Errorf("len(toasts) = %d, want 1", len(m.toasts))
	}
	if m.toasts[0].msg != "hello" {
		t.Errorf("toasts[0].msg = %q, want %q", m.toasts[0].msg, "hello")
	}
}

// TestPushToast_ReturnsCmd verifies that pushToast returns a non-nil Cmd.
func TestPushToast_ReturnsCmd(t *testing.T) {
	m := makeTestModelForToast(t)
	_, cmd := m.pushToast("hello")
	if cmd == nil {
		t.Error("pushToast should return a non-nil Cmd")
	}
}

// TestPushToast_SecondPushNoCmd verifies that a second push (queue already non-empty)
// returns a nil Cmd (tick already armed).
func TestPushToast_SecondPushNoCmd(t *testing.T) {
	m := makeTestModelForToast(t)
	m, _ = m.pushToast("first")
	_, cmd2 := m.pushToast("second")
	if cmd2 != nil {
		t.Error("second pushToast should return nil Cmd (tick already armed)")
	}
}

// TestExpireToasts_RemovesExpired verifies that expired toasts are pruned.
func TestExpireToasts_RemovesExpired(t *testing.T) {
	m := makeTestModelForToast(t)
	m.toasts = []toastEntry{
		{msg: "expired", expiresAt: time.Now().Add(-time.Second)},
	}
	m, _ = expireToastsImpl(m)
	if len(m.toasts) != 0 {
		t.Errorf("len(toasts) = %d, want 0 after expiry", len(m.toasts))
	}
}

// TestExpireToasts_KeepsLive verifies that live toasts are retained.
func TestExpireToasts_KeepsLive(t *testing.T) {
	m := makeTestModelForToast(t)
	m.toasts = []toastEntry{
		{msg: "live", expiresAt: time.Now().Add(5 * time.Second)},
	}
	m, _ = expireToastsImpl(m)
	if len(m.toasts) != 1 {
		t.Errorf("len(toasts) = %d, want 1 (live toast retained)", len(m.toasts))
	}
}

// TestExpireToasts_ReArmsForNextExpiry verifies that when toasts remain after pruning,
// a re-arm Cmd is returned.
func TestExpireToasts_ReArmsForNextExpiry(t *testing.T) {
	m := makeTestModelForToast(t)
	m.toasts = []toastEntry{
		{msg: "expired", expiresAt: time.Now().Add(-time.Second)},
		{msg: "live", expiresAt: time.Now().Add(5 * time.Second)},
	}
	m, cmd := expireToastsImpl(m)
	if len(m.toasts) != 1 {
		t.Errorf("len(toasts) = %d, want 1", len(m.toasts))
	}
	if cmd == nil {
		t.Error("expireToastsImpl should return a non-nil re-arm Cmd when toasts remain")
	}
}

// TestUserFriendlyError_Sentinels verifies each known sentinel maps to a non-empty,
// non-stack-trace string.
func TestUserFriendlyError_Sentinels(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrWorktreePathExists", git.ErrWorktreePathExists},
		{"ErrCmuxUnreachable", cmuxcli.ErrCmuxUnreachable},
		{"ErrConflict", tracker.ErrConflict},
		{"unknown error", errors.New("some internal error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := userFriendlyError(tt.err)
			if msg == "" {
				t.Errorf("userFriendlyError(%v) returned empty string", tt.err)
			}
			// Must not contain stack trace indicators.
			stackTraceIndicators := []string{"goroutine ", ".go:", "panic:"}
			for _, indicator := range stackTraceIndicators {
				if containsStr(msg, indicator) {
					t.Errorf("userFriendlyError result contains stack trace indicator %q: %q", indicator, msg)
				}
			}
		})
	}
}
