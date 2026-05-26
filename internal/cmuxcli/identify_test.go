package cmuxcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// containsStr reports whether s contains sub as a substring.
// (Local helper so tests don't import strings everywhere they need contains.)
func containsStr(s, sub string) bool {
	return strings.Contains(s, sub)
}

func TestIdentify_HappyPath(t *testing.T) {
	const payload = `{"caller":{"is_browser_surface":false,"pane_ref":"pane:1","surface_ref":"surface:1","surface_type":"terminal","tab_ref":"tab:1","window_ref":"window:1","workspace_ref":"workspace:1"},"focused":{"is_browser_surface":false,"pane_ref":"pane:5","surface_ref":"surface:5","surface_type":"terminal","tab_ref":"tab:5","window_ref":"window:1","workspace_ref":"workspace:2"},"socket_path":"/tmp/cmux.sock"}`

	dir := t.TempDir()
	// Use single quotes around the JSON literal so the shell does not interpret it.
	writeFakeCmux(t, dir, "#!/bin/sh\necho '"+payload+"'\n")
	prependPath(t, dir)

	got, err := Identify(context.Background())
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if got.Caller.WorkspaceRef != "workspace:1" {
		t.Errorf("caller.workspace_ref = %q, want workspace:1", got.Caller.WorkspaceRef)
	}
	if got.Focused.WorkspaceRef != "workspace:2" {
		t.Errorf("focused.workspace_ref = %q, want workspace:2", got.Focused.WorkspaceRef)
	}
	if got.SocketPath != "/tmp/cmux.sock" {
		t.Errorf("socket_path = %q, want /tmp/cmux.sock", got.SocketPath)
	}
}

func TestIdentify_ArgvContainsJsonAndIdentify(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\necho '{\"caller\":{},\"focused\":{},\"socket_path\":\"\"}'\n")
	prependPath(t, dir)

	_, _ = Identify(context.Background())
	data, _ := os.ReadFile(argvLog)
	argv := string(data)
	if !containsStr(argv, "--json") {
		t.Errorf("argv %q missing --json flag", argv)
	}
	if !containsStr(argv, "identify") {
		t.Errorf("argv %q missing identify subcommand", argv)
	}
}

func TestIdentify_ParseError(t *testing.T) {
	dir := t.TempDir()
	writeFakeCmux(t, dir, "#!/bin/sh\necho 'not json'\n")
	prependPath(t, dir)

	_, err := Identify(context.Background())
	if err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}
}
