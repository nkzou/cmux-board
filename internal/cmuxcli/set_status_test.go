package cmuxcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSetStatus_TrackerOnline_Argv(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\nexit 0\n")
	prependPath(t, dir)

	err := SetStatus(context.Background(), PillKeyTracker, "online", SetStatusOpts{})
	if err != nil {
		t.Fatalf("SetStatus error: %v", err)
	}
	data, _ := os.ReadFile(argvLog)
	argv := string(data)

	for _, want := range []string{"set-status", "tracker", "online", "--icon", "cloud", "--color", "#22c55e"} {
		if !containsStr(argv, want) {
			t.Errorf("argv %q missing expected token %q", argv, want)
		}
	}
}

func TestSetStatus_TrackerOffline_UsesOfflineColor(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\nexit 0\n")
	prependPath(t, dir)

	err := SetTrackerOffline(context.Background(), "14:30")
	if err != nil {
		t.Fatalf("SetTrackerOffline error: %v", err)
	}
	data, _ := os.ReadFile(argvLog)
	argv := string(data)

	if !containsStr(argv, "#f59e0b") {
		t.Errorf("argv %q missing tracker offline color #f59e0b", argv)
	}
	if !containsStr(argv, "offline (since 14:30)") {
		t.Errorf("argv %q missing offline value", argv)
	}
}

func TestSetStatus_CmuxUnreachable_UsesRedColor(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\nexit 0\n")
	prependPath(t, dir)

	_ = SetCmuxUnreachable(context.Background())
	data, _ := os.ReadFile(argvLog)
	argv := string(data)

	if !containsStr(argv, "#ef4444") {
		t.Errorf("argv %q missing cmux unreachable color #ef4444", argv)
	}
	if !containsStr(argv, "terminal") {
		t.Errorf("argv %q missing icon 'terminal'", argv)
	}
}

func TestSetStatus_ClaudeDegraded_UsesPurpleColor(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\nexit 0\n")
	prependPath(t, dir)

	_ = SetClaudeDegraded(context.Background())
	data, _ := os.ReadFile(argvLog)
	argv := string(data)

	if !containsStr(argv, "#a855f7") {
		t.Errorf("argv %q missing claude degraded color #a855f7", argv)
	}
	if !containsStr(argv, "sparkle") {
		t.Errorf("argv %q missing icon 'sparkle'", argv)
	}
}

func TestSetStatus_WithWorkspaceRef(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\nexit 0\n")
	prependPath(t, dir)

	_ = SetStatus(context.Background(), PillKeyTracker, "online",
		SetStatusOpts{WorkspaceRef: "workspace:3"})
	data, _ := os.ReadFile(argvLog)
	argv := string(data)

	if !containsStr(argv, "--workspace") || !containsStr(argv, "workspace:3") {
		t.Errorf("argv %q missing --workspace workspace:3 override", argv)
	}
}

func TestSetStatus_UnknownKey_ReturnsError(t *testing.T) {
	err := SetStatus(context.Background(), PillKey("unknown"), "val", SetStatusOpts{})
	if err == nil {
		t.Fatal("expected error on unknown pill key, got nil")
	}
}

func TestSetStatus_PriorityFlag(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\nexit 0\n")
	prependPath(t, dir)

	_ = SetStatus(context.Background(), PillKeyTracker, "online",
		SetStatusOpts{Priority: 10, HasPriority: true})
	data, _ := os.ReadFile(argvLog)
	argv := string(data)
	if !containsStr(argv, "--priority") || !containsStr(argv, "10") {
		t.Errorf("argv %q missing --priority 10", argv)
	}
}

func TestSetStatus_PriorityFlagOmittedWhenHasPriorityFalse(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\nexit 0\n")
	prependPath(t, dir)

	_ = SetStatus(context.Background(), PillKeyTracker, "online", SetStatusOpts{})
	data, _ := os.ReadFile(argvLog)
	argv := string(data)
	if containsStr(argv, "--priority") {
		t.Errorf("argv %q includes --priority but HasPriority was false", argv)
	}
}

func TestPillStyle_AllKeysPresent(t *testing.T) {
	for _, key := range []PillKey{PillKeyTracker, PillKeyCmux, PillKeyClaude} {
		style, ok := PillStyle(key)
		if !ok {
			t.Errorf("PillStyle(%q) returned ok=false", key)
		}
		if style.Icon == "" {
			t.Errorf("PillStyle(%q).Icon is empty", key)
		}
		if style.HealthyColor == "" || style.OfflineColor == "" {
			t.Errorf("PillStyle(%q) has empty color values", key)
		}
	}
}

func TestPillStyle_UnknownKey(t *testing.T) {
	_, ok := PillStyle(PillKey("nope"))
	if ok {
		t.Error("expected ok=false for unknown key")
	}
}
