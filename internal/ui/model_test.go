package ui

import (
	"testing"

	"github.com/kevin-zou/cmux-board/internal/config"
	"github.com/kevin-zou/cmux-board/internal/state"
)

// makeTestModel constructs a minimal Model backed by an in-memory store for testing.
func makeTestModel(t *testing.T) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	cfg := &config.Config{SchemaVersion: config.SchemaVersionCurrent}
	return NewModel(cfg, store)
}

// TestNewModel_InitialMode asserts that NewModel returns a model in ModeNormal.
func TestNewModel_InitialMode(t *testing.T) {
	m := makeTestModel(t)
	if m.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", m.mode)
	}
}

// TestNewModel_ApproachInputReady asserts that approachNameInput is initialized with
// the correct placeholder and CharLimit.
func TestNewModel_ApproachInputReady(t *testing.T) {
	m := makeTestModel(t)
	if m.approachNameInput.Placeholder != "approach name" {
		t.Errorf("Placeholder = %q, want %q", m.approachNameInput.Placeholder, "approach name")
	}
	if m.approachNameInput.CharLimit != 64 {
		t.Errorf("CharLimit = %d, want 64", m.approachNameInput.CharLimit)
	}
}

// TestModel_Init_ReturnsCmd asserts that Init returns a non-nil tea.Cmd.
func TestModel_Init_ReturnsCmd(t *testing.T) {
	m := makeTestModel(t)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() returned nil Cmd, want non-nil")
	}
}
