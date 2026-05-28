package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

// makeTestModelForUpdate creates a Model with an in-memory store for update tests.
func makeTestModelForUpdate(t *testing.T) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	cfg := &config.Config{SchemaVersion: config.SchemaVersionCurrent}
	return NewModel(cfg, store)
}

// TestUpdate_WindowSize verifies that a WindowSizeMsg stores width and height.
func TestUpdate_WindowSize(t *testing.T) {
	m := makeTestModelForUpdate(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	nm := next.(Model)
	if nm.width != 80 {
		t.Errorf("width = %d, want 80", nm.width)
	}
	if nm.height != 24 {
		t.Errorf("height = %d, want 24", nm.height)
	}
}

// TestUpdate_PollOKRefreshesSnapshot verifies that PollOKMsg clears pollFailedAt and
// re-derives the snapshot.
func TestUpdate_PollOKRefreshesSnapshot(t *testing.T) {
	m := makeTestModelForUpdate(t)
	// Pre-set a failure state that PollOK should clear.
	now := time.Now()
	m.pollFailedAt = &now
	m.pollErrCode = "5xx"

	next, _ := m.Update(PollOKMsg{Rev: 1})
	nm := next.(Model)

	if nm.pollFailedAt != nil {
		t.Error("pollFailedAt should be nil after PollOK")
	}
	if nm.pollErrCode != "" {
		t.Errorf("pollErrCode = %q, want empty", nm.pollErrCode)
	}
	if nm.snapshot == nil {
		t.Error("snapshot should be non-nil after PollOK")
	}
}

// TestUpdate_PollErrSetsState verifies that PollErrMsg records the failure time and code.
func TestUpdate_PollErrSetsState(t *testing.T) {
	m := makeTestModelForUpdate(t)
	now := time.Now()
	next, _ := m.Update(PollErrMsg{Code: "5xx", When: now})
	nm := next.(Model)

	if nm.pollFailedAt == nil {
		t.Fatal("pollFailedAt should be non-nil after PollErr")
	}
	if !nm.pollFailedAt.Equal(now) {
		t.Errorf("pollFailedAt = %v, want %v", nm.pollFailedAt, now)
	}
	if nm.pollErrCode != "5xx" {
		t.Errorf("pollErrCode = %q, want %q", nm.pollErrCode, "5xx")
	}
}

// TestUpdate_UnknownMsgNoop verifies that an unrecognized message type leaves the model unchanged.
func TestUpdate_UnknownMsgNoop(t *testing.T) {
	m := makeTestModelForUpdate(t)
	type unknownMsg struct{}
	next, cmd := m.Update(unknownMsg{})
	nm := next.(Model)

	if nm.mode != m.mode {
		t.Errorf("mode changed from %v to %v, want unchanged", m.mode, nm.mode)
	}
	if cmd != nil {
		t.Error("cmd should be nil for unknown message")
	}
}
