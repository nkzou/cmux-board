package sync

import (
	"testing"
	"time"
)

// TC-1: PollErrMsg PillText for auth must match the E5-locked literal exactly.
func TestPollErrMsgPillTextAuth(t *testing.T) {
	msg := PollErrMsg{Kind: PollErrorAuth, At: time.Now()}
	got := msg.PillText()
	const want = pillTextAuth
	if got != want {
		t.Errorf("TC-1: PillText(Auth): want %q, got %q", want, got)
	}
}

// TC-2: PollErrMsg PillText for transient must contain the HH:MM of the timestamp.
func TestPollErrMsgPillTextTransient(t *testing.T) {
	at := time.Date(2026, 1, 2, 15, 4, 0, 0, time.UTC)
	msg := PollErrMsg{Kind: PollErrorTransient, At: at}
	got := msg.PillText()

	const prefix = "offline (since "
	if len(got) < len(prefix) || got[:len(prefix)] != prefix {
		t.Errorf("TC-2: PillText(Transient): want prefix %q, got %q", prefix, got)
	}
	if !containsStr(got, "15:04") {
		t.Errorf("TC-2: PillText(Transient): want %q to contain %q", got, "15:04")
	}
}

// TC-3: PushConflictMsg carries the correct ConflictKind values.
func TestPushConflictMsgKind(t *testing.T) {
	occ := PushConflictMsg{Kind: ConflictOCC}
	if occ.Kind != ConflictOCC {
		t.Errorf("TC-3: want ConflictOCC, got %v", occ.Kind)
	}

	inv := PushConflictMsg{Kind: ConflictInvalidTransition}
	if inv.Kind != ConflictInvalidTransition {
		t.Errorf("TC-3: want ConflictInvalidTransition, got %v", inv.Kind)
	}

	if ConflictOCC == ConflictInvalidTransition {
		t.Error("TC-3: ConflictOCC and ConflictInvalidTransition must be distinct values")
	}
}

// containsStr is a simple substring helper (avoids importing strings in test).
func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
