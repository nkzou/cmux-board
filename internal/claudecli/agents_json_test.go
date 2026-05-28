package claudecli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeAgentsClaude writes a fake claude binary that outputs the given JSON on stdout.
// If exitCode != 0 it exits non-zero with an error message on stderr.
func writeFakeAgentsClaude(t *testing.T, dir, argvFile, jsonOutput string, exitCode int) {
	t.Helper()
	exitStr := "0"
	if exitCode != 0 {
		exitStr = "1"
	}
	// Escape single quotes in jsonOutput for shell safety.
	// For test purposes the JSON only contains double quotes and standard chars.
	script := "#!/bin/sh\n"
	script += "printf '%s\\n' \"$@\" > '" + argvFile + "'\n"
	if exitCode != 0 {
		script += "echo 'error: fake failure' >&2\n"
		script += "exit " + exitStr + "\n"
	} else {
		// Write JSON output; use cat with heredoc-style via printf.
		script += "cat << 'ENDJSON'\n" + jsonOutput + "\nENDJSON\n"
		script += "exit 0\n"
	}
	p := filepath.Join(dir, "claude")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("writeFakeAgentsClaude: %v", err)
	}
}

func TestAgents_TypicalOutput(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	jsonOutput := `[
  {"pid":1234,"cwd":"/worktrees/proj","kind":"background","startedAt":1700000001000,"sessionId":"4fea3626-0000-0000-0000-000000000001","name":"cmux-board-research-cd-then-bg","status":"busy"},
  {"pid":5678,"cwd":"/worktrees/proj2","kind":"interactive","startedAt":1700000002000,"sessionId":"3174068b-0000-0000-0000-000000000002"},
  {"pid":9999,"cwd":"/worktrees/proj3","kind":"background","startedAt":1700000003000,"sessionId":"eebf398b-0000-0000-0000-000000000003","name":"other","status":"idle"}
]`

	writeFakeAgentsClaude(t, dir, argvFile, jsonOutput, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	entries, err := Agents(context.Background(), AgentsArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}
	if entries[0].Kind != "background" {
		t.Errorf("entries[0].Kind = %q, want %q", entries[0].Kind, "background")
	}
	if entries[0].Name != "cmux-board-research-cd-then-bg" {
		t.Errorf("entries[0].Name = %q, want %q", entries[0].Name, "cmux-board-research-cd-then-bg")
	}
	if entries[1].Kind != "interactive" {
		t.Errorf("entries[1].Kind = %q, want %q", entries[1].Kind, "interactive")
	}
	if entries[1].Name != "" {
		t.Errorf("entries[1].Name = %q, want empty", entries[1].Name)
	}
	if entries[2].Status != "idle" {
		t.Errorf("entries[2].Status = %q, want %q", entries[2].Status, "idle")
	}
}

func TestAgents_UnknownStatus(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	jsonOutput := `[{"pid":1,"cwd":"/tmp","kind":"background","startedAt":1000,"sessionId":"aabb1122-0000-0000-0000-000000000001","status":"vaporized"}]`

	writeFakeAgentsClaude(t, dir, argvFile, jsonOutput, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	entries, err := Agents(context.Background(), AgentsArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Status != "unknown" {
		t.Errorf("Status = %q, want %q", entries[0].Status, "unknown")
	}
}

func TestAgents_MissingOptionalFields(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	// Only required fields: pid, cwd, kind, startedAt.
	jsonOutput := `[{"pid":1,"cwd":"/tmp","kind":"background","startedAt":0}]`

	writeFakeAgentsClaude(t, dir, argvFile, jsonOutput, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	entries, err := Agents(context.Background(), AgentsArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Name != "" {
		t.Errorf("Name = %q, want empty", entries[0].Name)
	}
	if entries[0].SessionID != "" {
		t.Errorf("SessionID = %q, want empty", entries[0].SessionID)
	}
	// Empty status is not normalized to "unknown".
	if entries[0].Status != "" {
		t.Errorf("Status = %q, want empty (not normalized)", entries[0].Status)
	}
}

func TestAgents_EmptyArray(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	writeFakeAgentsClaude(t, dir, argvFile, `[]`, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	entries, err := Agents(context.Background(), AgentsArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("len(entries) = %d, want 0", len(entries))
	}
}

func TestAgents_MalformedEntry(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	// First entry has pid as a string (invalid type); second is valid.
	jsonOutput := `[{"pid":"not-an-int","cwd":"/tmp"},{"pid":2,"cwd":"/tmp","kind":"background","startedAt":1}]`

	writeFakeAgentsClaude(t, dir, argvFile, jsonOutput, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	entries, err := Agents(context.Background(), AgentsArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1 (malformed entry skipped)", len(entries))
	}
	if entries[0].PID != 2 {
		t.Errorf("PID = %d, want 2", entries[0].PID)
	}
}

func TestAgents_WithCWDFilter(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	writeFakeAgentsClaude(t, dir, argvFile, `[]`, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := Agents(context.Background(), AgentsArgs{CWD: "/private/tmp/test-worktree"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	argvBytes, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	args := strings.Split(strings.TrimRight(string(argvBytes), "\n"), "\n")

	foundCWD := false
	for i, arg := range args {
		if arg == "--cwd" {
			if i+1 < len(args) && args[i+1] == "/private/tmp/test-worktree" {
				foundCWD = true
			}
		}
	}
	if !foundCWD {
		t.Errorf("argv does not contain --cwd /private/tmp/test-worktree; got: %v", args)
	}
}

func TestAgents_WithoutCWDFilter(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	writeFakeAgentsClaude(t, dir, argvFile, `[]`, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := Agents(context.Background(), AgentsArgs{CWD: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	argvBytes, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	args := strings.Split(strings.TrimRight(string(argvBytes), "\n"), "\n")

	for _, arg := range args {
		if arg == "--cwd" {
			t.Errorf("argv contains --cwd when CWD is empty; got: %v", args)
		}
	}
}

func TestAgents_ExecFailure(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	writeFakeAgentsClaude(t, dir, argvFile, "", 1)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := Agents(context.Background(), AgentsArgs{})
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
}

func TestFindByShortID_Match(t *testing.T) {
	entries := []AgentEntry{
		{SessionID: "4fea3626-0000-0000-0000-000000000001", StartedAt: 1000},
		{SessionID: "3174068b-0000-0000-0000-000000000002", StartedAt: 2000},
		{SessionID: "eebf398b-0000-0000-0000-000000000003", StartedAt: 3000},
	}
	match := FindByShortID(entries, "3174068b")
	if match == nil {
		t.Fatal("expected match, got nil")
	}
	if match.SessionID != "3174068b-0000-0000-0000-000000000002" {
		t.Errorf("SessionID = %q, want %q", match.SessionID, "3174068b-0000-0000-0000-000000000002")
	}
}

func TestFindByShortID_NoMatch(t *testing.T) {
	entries := []AgentEntry{
		{SessionID: "4fea3626-0000-0000-0000-000000000001"},
	}
	match := FindByShortID(entries, "deadbeef")
	if match != nil {
		t.Errorf("expected nil, got %+v", match)
	}
}

func TestFindByShortID_TiebreakNewest(t *testing.T) {
	entries := []AgentEntry{
		{SessionID: "12345678-older", StartedAt: 100},
		{SessionID: "12345678-newer", StartedAt: 200},
	}
	match := FindByShortID(entries, "12345678")
	if match == nil {
		t.Fatal("expected match, got nil")
	}
	if match.StartedAt != 200 {
		t.Errorf("tiebreak: got StartedAt %d, want 200 (newer)", match.StartedAt)
	}
}

func TestFindByActIDShort_SubstringMatch(t *testing.T) {
	entries := []AgentEntry{
		{Name: "cmux-board:PROJ-42:cafef00d", SessionID: "aaa"},
		{Name: "other-session", SessionID: "bbb"},
		{Name: "cmux-board:PROJ-1:deadbeef", SessionID: "ccc"},
	}
	result := FindByActIDShort(entries, "cafef00d")
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if result[0].SessionID != "aaa" {
		t.Errorf("SessionID = %q, want %q", result[0].SessionID, "aaa")
	}
}

func TestAgents_MacOSPathNormalization(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	// Entry has /private/tmp cwd even though --cwd was passed as /tmp.
	jsonOutput := `[{"pid":1,"cwd":"/private/tmp/myworktree","kind":"background","startedAt":1000,"sessionId":"abcd1234-0000-0000-0000-000000000001"}]`

	writeFakeAgentsClaude(t, dir, argvFile, jsonOutput, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	entries, err := Agents(context.Background(), AgentsArgs{CWD: "/tmp/myworktree"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// We just verify the entry is returned (cwd normalization is claude's responsibility).
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].CWD != "/private/tmp/myworktree" {
		t.Errorf("CWD = %q, want /private/tmp/myworktree", entries[0].CWD)
	}
}
