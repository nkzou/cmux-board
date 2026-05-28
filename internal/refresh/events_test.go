package refresh

import (
	"testing"
	"time"
)

// TC-1: PillText for auth must match the E5-locked literal exactly.
func TestPillTextAuth(t *testing.T) {
	t.Parallel()
	got := PillText(PollErrorAuth, time.Now())
	const want = pillTextAuth
	if got != want {
		t.Errorf("TC-1: PillText(Auth): want %q, got %q", want, got)
	}
}

// TC-2: PillText for transient must contain the HH:MM of the timestamp.
func TestPillTextTransient(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 1, 2, 15, 4, 0, 0, time.UTC)
	got := PillText(PollErrorTransient, at)

	const prefix = "offline (since "
	if len(got) < len(prefix) || got[:len(prefix)] != prefix {
		t.Errorf("TC-2: PillText(Transient): want prefix %q, got %q", prefix, got)
	}
	if !containsStr(got, "15:04") {
		t.Errorf("TC-2: PillText(Transient): want %q to contain %q", got, "15:04")
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
