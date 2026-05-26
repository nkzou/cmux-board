package cmuxcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWorkspaceWithLayout_HappyPath(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\necho 'OK workspace:4'\n")
	prependPath(t, dir)

	ref, err := NewWorkspaceWithLayout(context.Background(), NewWorkspaceArgs{
		Name:               "PROJ-42 [abc12345]",
		CWD:                "/tmp/worktrees/openkanban-PROJ-42-main-abc12345",
		AgentAttachCommand: "claude attach 7c5dcf5d",
		ShellCommand:       "",
	})
	if err != nil {
		t.Fatalf("NewWorkspaceWithLayout error: %v", err)
	}
	if ref != "workspace:4" {
		t.Errorf("ref = %q, want workspace:4", ref)
	}

	data, _ := os.ReadFile(argvLog)
	argv := string(data)

	// Argv snapshot assertions.
	for _, want := range []string{"new-workspace", "--name", "PROJ-42 [abc12345]",
		"--cwd", "--layout"} {
		if !containsStr(argv, want) {
			t.Errorf("argv %q missing expected token %q", argv, want)
		}
	}
	// Layout JSON must include the agent attach command.
	if !containsStr(argv, "claude attach 7c5dcf5d") {
		t.Errorf("argv %q missing agent attach command in layout", argv)
	}
	// Must be "horizontal" split.
	if !containsStr(argv, "horizontal") {
		t.Errorf("argv %q missing direction:horizontal in layout", argv)
	}
}

func TestNewWorkspaceWithLayout_FocusFlag(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\necho 'OK workspace:5'\n")
	prependPath(t, dir)

	_, err := NewWorkspaceWithLayout(context.Background(), NewWorkspaceArgs{
		Name:               "test",
		CWD:                "/tmp",
		AgentAttachCommand: "claude attach aabbccdd",
		Focus:              true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(argvLog)
	if !containsStr(string(data), "--focus") {
		t.Errorf("argv %q missing --focus flag when Focus=true", string(data))
	}
}

func TestNewWorkspaceWithLayout_ParseError_NotOK(t *testing.T) {
	dir := t.TempDir()
	writeFakeCmux(t, dir, "#!/bin/sh\necho 'ERR something went wrong'\n")
	prependPath(t, dir)

	_, err := NewWorkspaceWithLayout(context.Background(), NewWorkspaceArgs{
		Name: "test", CWD: "/tmp", AgentAttachCommand: "claude attach aabbccdd",
	})
	if err == nil {
		t.Fatal("expected error on non-OK response, got nil")
	}
}

func TestNewWorkspaceWithLayout_NoDestructiveFlags(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\necho 'OK workspace:4'\n")
	prependPath(t, dir)

	_, _ = NewWorkspaceWithLayout(context.Background(), NewWorkspaceArgs{
		Name: "test", CWD: "/tmp", AgentAttachCommand: "claude attach aabbccdd",
	})
	data, _ := os.ReadFile(argvLog)
	argv := strings.ToLower(string(data))
	for _, forbidden := range []string{"close-others", "close-window", "workspace-action", "kill"} {
		if containsStr(argv, forbidden) {
			t.Errorf("argv %q contains forbidden destructive token %q (additive-only violation)",
				argv, forbidden)
		}
	}
}

func TestBuildLayoutJSON_TwoPanes(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\necho 'OK workspace:4'\n")
	prependPath(t, dir)

	_, _ = NewWorkspaceWithLayout(context.Background(), NewWorkspaceArgs{
		Name:               "PROJ-42 [abc12345]",
		CWD:                "/tmp",
		AgentAttachCommand: "claude attach 7c5dcf5d",
		ShellCommand:       "bash",
	})
	data, _ := os.ReadFile(argvLog)
	argv := string(data)
	if !containsStr(argv, "bash") {
		t.Errorf("layout JSON missing shell command 'bash' in argv: %q", argv)
	}
	if !containsStr(argv, "split") {
		t.Errorf("layout JSON missing split field in argv: %q", argv)
	}
	if !containsStr(argv, "horizontal") {
		t.Errorf("layout JSON missing direction:horizontal: %q", argv)
	}
}

func TestParseWorkspaceRef_OKLine(t *testing.T) {
	ref, err := parseWorkspaceRef("OK workspace:7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "workspace:7" {
		t.Errorf("ref = %q, want workspace:7", ref)
	}
}

func TestParseWorkspaceRef_EmptyAfterOK(t *testing.T) {
	_, err := parseWorkspaceRef("OK ")
	if err == nil {
		t.Fatal("want error on empty ref after OK, got nil")
	}
}

func TestParseWorkspaceRef_NonOKLine(t *testing.T) {
	_, err := parseWorkspaceRef("ERR something failed")
	if err == nil {
		t.Fatal("want error on non-OK line, got nil")
	}
}
