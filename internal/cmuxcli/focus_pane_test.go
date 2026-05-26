package cmuxcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFocusPane_HappyPath(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\nexit 0\n")
	prependPath(t, dir)

	err := FocusPane(context.Background(), "workspace:4", "pane:7")
	if err != nil {
		t.Fatalf("FocusPane error: %v", err)
	}

	data, _ := os.ReadFile(argvLog)
	argv := string(data)

	for _, want := range []string{"focus-pane", "--workspace", "workspace:4", "--pane", "pane:7"} {
		if !containsStr(argv, want) {
			t.Errorf("argv %q missing expected token %q", argv, want)
		}
	}
}

// TestFocusPane_NegativeArgv_NoBareFreeFocus is the Codex Finding 8 regression guard.
// The argv must NOT start with "focus " — the bare "focus" subcommand does not exist
// in cmux 0.64.7 (RESEARCH §7). Only "focus-pane" is valid.
func TestFocusPane_NegativeArgv_NoBareFreeFocus(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\nexit 0\n")
	prependPath(t, dir)

	_ = FocusPane(context.Background(), "workspace:4", "pane:7")

	data, _ := os.ReadFile(argvLog)
	argv := strings.TrimSpace(string(data))

	tokens := strings.Fields(argv)
	if len(tokens) == 0 {
		t.Fatal("argv was empty")
	}
	// First token must be the literal "focus-pane", not "focus".
	if tokens[0] == "focus" {
		t.Errorf("argv %q uses bare 'focus' subcommand — this does not exist in cmux 0.64.7"+
			" (Codex Finding 8 / RESEARCH §7); use 'focus-pane --workspace ... --pane ...' instead",
			argv)
	}
	if tokens[0] != "focus-pane" {
		t.Errorf("argv first token = %q, want focus-pane (Codex Finding 8)", tokens[0])
	}
}

// TestFocusPane_NegativeArgv_NoWorkspaceActionFocus asserts that workspace-action is
// never used as a focus mechanism (RESEARCH §7: "No focus action documented").
func TestFocusPane_NegativeArgv_NoWorkspaceActionFocus(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\nexit 0\n")
	prependPath(t, dir)

	_ = FocusPane(context.Background(), "workspace:4", "pane:7")

	data, _ := os.ReadFile(argvLog)
	argv := strings.ToLower(string(data))
	if containsStr(argv, "workspace-action") {
		t.Errorf("argv %q uses workspace-action for focus — forbidden (RESEARCH §7)", argv)
	}
}

func TestFocusPane_EmptyWorkspaceRef(t *testing.T) {
	err := FocusPane(context.Background(), "", "pane:7")
	if err == nil {
		t.Fatal("expected error on empty wsRef, got nil")
	}
}

func TestFocusPane_EmptyPaneRef(t *testing.T) {
	err := FocusPane(context.Background(), "workspace:4", "")
	if err == nil {
		t.Fatal("expected error on empty paneRef, got nil")
	}
}

func TestFocusPane_ExitError(t *testing.T) {
	dir := t.TempDir()
	writeFakeCmux(t, dir, "#!/bin/sh\nexit 1\n")
	prependPath(t, dir)

	err := FocusPane(context.Background(), "workspace:4", "pane:7")
	if err == nil {
		t.Fatal("expected error on cmux exit 1, got nil")
	}
}
