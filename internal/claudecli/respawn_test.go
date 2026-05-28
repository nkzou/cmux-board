package claudecli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeIsOrphanClaude writes a fake claude binary for orphan-detection tests.
// The binary writes its argv to argvFile and outputs jsonOutput to stdout.
func writeFakeIsOrphanClaude(t *testing.T, dir, argvFile, jsonOutput string, exitCode int) {
	t.Helper()
	exitStr := "0"
	if exitCode != 0 {
		exitStr = "1"
	}
	script := "#!/bin/sh\n"
	script += "printf '%s\\n' \"$@\" > '" + argvFile + "'\n"
	if exitCode != 0 {
		script += "echo 'error: fake failure' >&2\n"
		script += "exit " + exitStr + "\n"
	} else {
		script += "cat << 'ENDJSON'\n" + jsonOutput + "\nENDJSON\n"
		script += "exit 0\n"
	}
	p := filepath.Join(dir, "claude")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("writeFakeIsOrphanClaude: %v", err)
	}
}

func TestIsOrphan_SessionPresent(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	jsonOutput := `[{"pid":1,"cwd":"/tmp/test-wt","kind":"background","startedAt":1000,"sessionId":"3174068b-6361-416e-ac44-e0bbdd35de13","status":"idle"}]`
	writeFakeIsOrphanClaude(t, dir, argvFile, jsonOutput, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	orphan, err := IsOrphan(context.Background(), "/tmp/test-wt", "3174068b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if orphan {
		t.Error("expected not orphan (session present), got true")
	}
}

func TestIsOrphan_SessionAbsent(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	writeFakeIsOrphanClaude(t, dir, argvFile, `[]`, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	orphan, err := IsOrphan(context.Background(), "/tmp/test-wt", "deadbeef")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !orphan {
		t.Error("expected orphan (empty list), got false")
	}
}

func TestIsOrphan_SessionAbsent_OtherSessionsPresent(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	jsonOutput := `[
  {"pid":1,"cwd":"/tmp","kind":"background","startedAt":1000,"sessionId":"aaaaaaaa-0000-0000-0000-000000000001"},
  {"pid":2,"cwd":"/tmp","kind":"background","startedAt":2000,"sessionId":"bbbbbbbb-0000-0000-0000-000000000002"}
]`
	writeFakeIsOrphanClaude(t, dir, argvFile, jsonOutput, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	orphan, err := IsOrphan(context.Background(), "/tmp/test-wt", "deadbeef")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !orphan {
		t.Error("expected orphan (short_id not in list), got false")
	}
}

func TestIsOrphan_EmptyShortID(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	// Write a fake binary that would record a call if invoked.
	writeFakeIsOrphanClaude(t, dir, argvFile, `[]`, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	orphan, err := IsOrphan(context.Background(), "/tmp/test-wt", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !orphan {
		t.Error("expected orphan for empty short_id, got false")
	}

	// Verify the fake claude binary was NOT invoked (argvFile should not exist).
	if _, statErr := os.Stat(argvFile); statErr == nil {
		t.Error("fake claude was invoked for empty shortID; it should not be called")
	}
}

func TestIsOrphan_AgentsError(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	writeFakeIsOrphanClaude(t, dir, argvFile, "", 1)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	orphan, err := IsOrphan(context.Background(), "/tmp/test-wt", "deadbeef")
	if err == nil {
		t.Fatal("expected error from agents exec failure, got nil")
	}
	if orphan {
		t.Error("expected false (not orphan) on error, got true")
	}
}

func TestIsOrphan_CWDPassedThrough(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	writeFakeIsOrphanClaude(t, dir, argvFile, `[]`, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, _ = IsOrphan(context.Background(), "/private/tmp/my-worktree", "3174068b")

	argvBytes, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	args := strings.Split(strings.TrimRight(string(argvBytes), "\n"), "\n")

	foundCWD := false
	for i, arg := range args {
		if arg == "--cwd" && i+1 < len(args) && args[i+1] == "/private/tmp/my-worktree" {
			foundCWD = true
		}
	}
	if !foundCWD {
		t.Errorf("argv does not contain --cwd /private/tmp/my-worktree; got: %v", args)
	}
}

func TestRespawn_DelegatesToLaunchBackground(t *testing.T) {
	dir := t.TempDir()
	worktree := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	cwdFile := filepath.Join(dir, "cwd.txt")

	stdout := "backgrounded \xc2\xb7 cafef00d \xc2\xb7 cmux-board:PROJ-42:deadbeef\n"
	writeFakeBGClaude(t, dir, argvFile, cwdFile, stdout, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	result, err := Respawn(context.Background(), BGArgs{
		Worktree: worktree,
		Name:     "cmux-board:PROJ-42:deadbeef",
		Prompt:   "starter prompt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ShortID != "cafef00d" {
		t.Errorf("ShortID = %q, want %q", result.ShortID, "cafef00d")
	}
	if result.Name != "cmux-board:PROJ-42:deadbeef" {
		t.Errorf("Name = %q, want %q", result.Name, "cmux-board:PROJ-42:deadbeef")
	}
}

// TestRespawn_NegativeArgv_NoCwd is the regression guard for Respawn → LaunchBackground.
// Verifies that --cwd is NEVER present in argv even through the delegation chain.
func TestRespawn_NegativeArgv_NoCwd(t *testing.T) {
	dir := t.TempDir()
	worktree := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	cwdFile := filepath.Join(dir, "cwd.txt")

	stdout := "backgrounded \xc2\xb7 abcdef12\n"
	writeFakeBGClaude(t, dir, argvFile, cwdFile, stdout, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := Respawn(context.Background(), BGArgs{
		Worktree: worktree,
		Name:     "test",
		Prompt:   "prompt",
	})
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
			t.Errorf("REGRESSION: Respawn argv contains --cwd; must use cmd.Dir. Full argv: %v", args)
		}
	}
}

// TestRespawn_CmdDirSet verifies cmd.Dir is set correctly through the Respawn → LaunchBackground chain.
func TestRespawn_CmdDirSet(t *testing.T) {
	dir := t.TempDir()
	worktree := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	cwdFile := filepath.Join(dir, "cwd.txt")

	stdout := "backgrounded \xc2\xb7 11223344\n"
	writeFakeBGClaude(t, dir, argvFile, cwdFile, stdout, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := Respawn(context.Background(), BGArgs{
		Worktree: worktree,
		Name:     "test",
		Prompt:   "prompt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cwdBytes, err := os.ReadFile(cwdFile)
	if err != nil {
		t.Fatalf("read cwd: %v", err)
	}
	capturedCWD := string(cwdBytes)
	resolvedCapture, _ := filepath.EvalSymlinks(capturedCWD)
	resolvedWorktree, _ := filepath.EvalSymlinks(worktree)
	if resolvedCapture != resolvedWorktree {
		t.Errorf("cmd.Dir mismatch via Respawn: got %q, want %q", capturedCWD, worktree)
	}
}
