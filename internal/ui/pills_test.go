package ui

import (
	"testing"
	"time"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

// countSetStatusCalls runs emitPill and returns how many non-nil Cmds were returned.
// It does NOT execute the Cmds (which would call the real cmux binary).
func countEmitCmds(cmds []interface{ cmd() interface{} }) int {
	// Unused helper — we count directly below.
	return 0
}

// makeTestModelForPills builds a minimal Model for pill tests.
func makeTestModelForPills(t *testing.T) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	cfg := &config.Config{SchemaVersion: config.SchemaVersionCurrent}
	return NewModel(cfg, store)
}

// TestEmitPill_DebounceSuppressDuplicate verifies that calling emitPill twice with the
// same key+text within the debounce window returns a Cmd only on the first call.
func TestEmitPill_DebounceSuppressDuplicate(t *testing.T) {
	m := makeTestModelForPills(t)

	m, cmd1 := m.emitPill(PillKeyTracker, PillTrackerOK)
	_, cmd2 := m.emitPill(PillKeyTracker, PillTrackerOK)

	if cmd1 == nil {
		t.Error("first emitPill should return a non-nil Cmd")
	}
	if cmd2 != nil {
		t.Error("second emitPill with same key+text within debounce window should return nil Cmd")
	}
}

// TestEmitPill_DifferentValueBypassesDebounce verifies that changing the text bypasses
// the debounce window even if called immediately.
func TestEmitPill_DifferentValueBypassesDebounce(t *testing.T) {
	m := makeTestModelForPills(t)

	m, cmd1 := m.emitPill(PillKeyTracker, PillTrackerOK)
	_, cmd2 := m.emitPill(PillKeyTracker, PillTrackerReauth) // different text

	if cmd1 == nil {
		t.Error("first emitPill should return a non-nil Cmd")
	}
	if cmd2 == nil {
		t.Error("second emitPill with different text should return a non-nil Cmd")
	}
}

// TestEmitPill_TimeoutResetsDebounce verifies that after the debounce window passes,
// the same key+text is re-emitted. We simulate this by backdating lastEmitted.
func TestEmitPill_TimeoutResetsDebounce(t *testing.T) {
	m := makeTestModelForPills(t)

	// First emit.
	m, cmd1 := m.emitPill(PillKeyTracker, PillTrackerOK)
	if cmd1 == nil {
		t.Fatal("first emit should produce a Cmd")
	}

	// Backdate the lastEmitted so the debounce window appears to have expired.
	m.trackerPill.lastEmitted = time.Now().Add(-2 * pillDebounceWindow)

	// Second emit with same text — should NOT be suppressed.
	_, cmd2 := m.emitPill(PillKeyTracker, PillTrackerOK)
	if cmd2 == nil {
		t.Error("emitPill after debounce window should return a non-nil Cmd")
	}
}

// TestPillText_TrackingOfflineSince verifies the format of the offline-since string.
func TestPillText_TrackingOfflineSince(t *testing.T) {
	// Use a fixed time so the test is deterministic.
	when := time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC)
	got := trackerOfflineText(when)

	if got == "" {
		t.Fatal("trackerOfflineText returned empty string")
	}
	// Must contain the base prefix and the since time.
	wantContains := []string{PillTrackerOffline, "(since 14:30)"}
	for _, want := range wantContains {
		if !containsStr(got, want) {
			t.Errorf("trackerOfflineText(%v) = %q, missing %q", when, got, want)
		}
	}
}

// TestPollErr_401_MapsToReauth verifies that a 401 error code emits PillTrackerReauth.
func TestPollErr_401_MapsToReauth(t *testing.T) {
	m := makeTestModelForPills(t)
	m, _ = m.handlePollErr(PollErrMsg{Code: "401", When: time.Now()})
	if m.trackerPill.text != PillTrackerReauth {
		t.Errorf("trackerPill.text = %q, want %q", m.trackerPill.text, PillTrackerReauth)
	}
	if !containsStr(m.trackerPill.text, "run cmux-board init --force") {
		t.Errorf("reauth text missing required substring, got: %q", m.trackerPill.text)
	}
}

// containsStr is a simple substring helper.
func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
