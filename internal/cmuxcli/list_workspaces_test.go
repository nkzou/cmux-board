package cmuxcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const listWorkspacesJSON = `{
  "window_ref": "window:1",
  "workspaces": [
    {
      "ref": "workspace:1",
      "title": "release suffix",
      "current_directory": "/Users/kevin.zou/git/cmux-board",
      "selected": true,
      "pinned": false,
      "index": 0
    },
    {
      "ref": "workspace:2",
      "title": "PROJ-42 [abc12345]",
      "current_directory": "/Users/kevin.zou/code/worktrees/my-service-PROJ-42-main-abc12345",
      "selected": false,
      "pinned": false,
      "index": 1
    }
  ]
}`

func TestListWorkspaces_HappyPath(t *testing.T) {
	dir := t.TempDir()
	// Write the JSON to a file, then have the script cat it (avoids quoting issues).
	jsonPath := filepath.Join(dir, "payload.json")
	if err := os.WriteFile(jsonPath, []byte(listWorkspacesJSON), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	writeFakeCmux(t, dir, "#!/bin/sh\ncat '"+jsonPath+"'\n")
	prependPath(t, dir)

	workspaces, err := ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListWorkspaces error: %v", err)
	}
	if len(workspaces) != 2 {
		t.Fatalf("got %d workspaces, want 2", len(workspaces))
	}
	if workspaces[0].Ref != "workspace:1" {
		t.Errorf("workspaces[0].Ref = %q, want workspace:1", workspaces[0].Ref)
	}
	if workspaces[1].Title != "PROJ-42 [abc12345]" {
		t.Errorf("workspaces[1].Title = %q, want PROJ-42 [abc12345]", workspaces[1].Title)
	}
}

func TestListWorkspaces_ArgvContainsJsonAndListWorkspaces(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\necho '{\"window_ref\":\"window:1\",\"workspaces\":[]}'\n")
	prependPath(t, dir)

	_, _ = ListWorkspaces(context.Background())
	data, _ := os.ReadFile(argvLog)
	argv := string(data)
	if !containsStr(argv, "--json") {
		t.Errorf("argv %q missing --json flag", argv)
	}
	if !containsStr(argv, "list-workspaces") {
		t.Errorf("argv %q missing list-workspaces subcommand", argv)
	}
}

// TestIsOrphan_MissingRef asserts that a cached workspace ID not in the live list
// is correctly identified as an orphan — the ONLY supported orphan-detection path.
func TestIsOrphan_MissingRef(t *testing.T) {
	workspaces := []Workspace{
		{Ref: "workspace:1"},
		{Ref: "workspace:3"},
	}
	if !IsOrphan("workspace:2", workspaces) {
		t.Error("expected workspace:2 to be an orphan")
	}
}

func TestIsOrphan_PresentRef(t *testing.T) {
	workspaces := []Workspace{
		{Ref: "workspace:1"},
		{Ref: "workspace:2"},
	}
	if IsOrphan("workspace:1", workspaces) {
		t.Error("expected workspace:1 NOT to be an orphan")
	}
}

// TestNoCurrentDirectoryAdoption verifies that current_directory is only available
// for logging, not for workspace adoption. A workspace whose current_directory matches
// a target worktree path is NOT returned as a match by IsOrphan (which only matches by ref).
// This confirms the adoption-by-heuristic path does not exist in the API surface (E8).
func TestNoCurrentDirectoryAdoption(t *testing.T) {
	targetWorktree := "/Users/kevin.zou/code/worktrees/my-service-PROJ-42-main-abc12345"
	workspaces := []Workspace{
		{
			Ref:              "workspace:99",
			Title:            "foreign workspace",
			CurrentDirectory: targetWorktree, // matches our path — but we must NOT adopt it
		},
	}
	// Our cached ref is "workspace:2" which is NOT in the list.
	// Correct behavior: mark as orphan regardless of current_directory match.
	if !IsOrphan("workspace:2", workspaces) {
		t.Error("expected workspace:2 to be an orphan even though a foreign workspace " +
			"has a matching current_directory — adoption by current_directory is forbidden (E8)")
	}
}

func TestListWorkspaces_ParseError(t *testing.T) {
	dir := t.TempDir()
	writeFakeCmux(t, dir, "#!/bin/sh\necho 'not json'\n")
	prependPath(t, dir)

	_, err := ListWorkspaces(context.Background())
	if err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}
}
