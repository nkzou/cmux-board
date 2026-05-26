package cmuxcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const listPanesJSON = `{
  "panes": [
    { "ref": "pane:7", "workspace_ref": "workspace:4", "index": 0, "active": true },
    { "ref": "pane:8", "workspace_ref": "workspace:4", "index": 1, "active": false }
  ]
}`

func TestListPanes_HappyPath(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "payload.json")
	if err := os.WriteFile(jsonPath, []byte(listPanesJSON), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	writeFakeCmux(t, dir, "#!/bin/sh\ncat '"+jsonPath+"'\n")
	prependPath(t, dir)

	panes, err := ListPanes(context.Background(), "workspace:4")
	if err != nil {
		t.Fatalf("ListPanes error: %v", err)
	}
	if len(panes) != 2 {
		t.Fatalf("got %d panes, want 2", len(panes))
	}
	if panes[0].Ref != "pane:7" {
		t.Errorf("panes[0].Ref = %q, want pane:7", panes[0].Ref)
	}
	if panes[1].Ref != "pane:8" {
		t.Errorf("panes[1].Ref = %q, want pane:8", panes[1].Ref)
	}
}

func TestListPanes_ArgvContainsWorkspaceRef(t *testing.T) {
	dir := t.TempDir()
	argvLog := filepath.Join(dir, "argv.txt")
	writeFakeCmux(t, dir, "#!/bin/sh\necho \"$@\" > \""+argvLog+"\"\necho '{\"panes\":[]}'\n")
	prependPath(t, dir)

	_, _ = ListPanes(context.Background(), "workspace:4")
	data, _ := os.ReadFile(argvLog)
	argv := string(data)

	for _, want := range []string{"--json", "list-panes", "--workspace", "workspace:4"} {
		if !containsStr(argv, want) {
			t.Errorf("argv %q missing expected token %q", argv, want)
		}
	}
}

func TestListPanes_EmptyWorkspaceRefErrors(t *testing.T) {
	_, err := ListPanes(context.Background(), "")
	if err == nil {
		t.Fatal("want error on empty wsRef")
	}
}

func TestAgentPaneRef_ReturnsIndex0Ref(t *testing.T) {
	panes := []Pane{
		{Ref: "pane:7", Index: 0, Active: true},
		{Ref: "pane:8", Index: 1, Active: false},
	}
	ref, err := AgentPaneRef(panes)
	if err != nil {
		t.Fatalf("AgentPaneRef error: %v", err)
	}
	if ref != "pane:7" {
		t.Errorf("AgentPaneRef = %q, want pane:7", ref)
	}
}

func TestAgentPaneRef_EmptyList(t *testing.T) {
	_, err := AgentPaneRef(nil)
	if err == nil {
		t.Fatal("expected error on empty pane list, got nil")
	}
}

func TestAgentPaneRef_NoIndex0(t *testing.T) {
	panes := []Pane{
		{Ref: "pane:9", Index: 1},
		{Ref: "pane:10", Index: 2},
	}
	_, err := AgentPaneRef(panes)
	if err == nil {
		t.Fatal("expected error when no index-0 pane, got nil")
	}
}

func TestListPanes_ParseError(t *testing.T) {
	dir := t.TempDir()
	writeFakeCmux(t, dir, "#!/bin/sh\necho 'not json'\n")
	prependPath(t, dir)

	_, err := ListPanes(context.Background(), "workspace:4")
	if err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}
}
