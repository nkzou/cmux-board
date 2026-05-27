package claudecli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeBG writes a fake claude binary that:
// - Writes its own argv (joined with NUL) to argvFile.
// - Writes its working directory (from $PWD) to cwdFile.
// - Prints stdout to its own stdout.
// - Exits 0.
//
// argvFile and cwdFile paths must be single-quoted-safe (no single quotes in path).
func writeFakeBGClaude(t *testing.T, dir, argvFile, cwdFile, stdout string, exitCode int) {
	t.Helper()
	exitStr := "0"
	if exitCode != 0 {
		exitStr = "1"
	}
	script := "#!/bin/sh\n"
	// Write argv (elements joined by newline) to argvFile.
	script += "printf '%s\\n' \"$@\" > '" + argvFile + "'\n"
	// Write $PWD to cwdFile.
	script += "printf '%s' \"$PWD\" > '" + cwdFile + "'\n"
	if exitCode != 0 {
		script += "echo 'error: fake failure' >&2\n"
		script += "exit " + exitStr + "\n"
	} else {
		// Use printf to avoid adding extra newlines.
		script += "printf '%s' '" + stdout + "'\n"
		script += "exit 0\n"
	}
	p := filepath.Join(dir, "claude")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("writeFakeBGClaude: %v", err)
	}
}

func TestLaunchBackground_WithName(t *testing.T) {
	dir := t.TempDir()
	worktree := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	cwdFile := filepath.Join(dir, "cwd.txt")

	stdout := "warning: --bg manages the session id; ignoring --session-id (use --resume <id> to continue an existing session)\n" +
		"backgrounded \xc2\xb7 3174068b \xc2\xb7 cmux-board:PROJ-42:deadbeef\n" +
		"  claude agents             list sessions\n"

	writeFakeBGClaude(t, dir, argvFile, cwdFile, stdout, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	result, err := LaunchBackground(context.Background(), BGArgs{
		Worktree: worktree,
		Name:     "cmux-board:PROJ-42:deadbeef",
		Prompt:   "starter prompt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ShortID != "3174068b" {
		t.Errorf("ShortID = %q, want %q", result.ShortID, "3174068b")
	}
	if result.Name != "cmux-board:PROJ-42:deadbeef" {
		t.Errorf("Name = %q, want %q", result.Name, "cmux-board:PROJ-42:deadbeef")
	}
	_ = argvFile
	_ = cwdFile
}

func TestLaunchBackground_WithoutName(t *testing.T) {
	dir := t.TempDir()
	worktree := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	cwdFile := filepath.Join(dir, "cwd.txt")

	stdout := "warning: something\n" +
		"backgrounded \xc2\xb7 eebf398b\n"

	writeFakeBGClaude(t, dir, argvFile, cwdFile, stdout, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	result, err := LaunchBackground(context.Background(), BGArgs{
		Worktree: worktree,
		Name:     "test",
		Prompt:   "prompt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ShortID != "eebf398b" {
		t.Errorf("ShortID = %q, want %q", result.ShortID, "eebf398b")
	}
	if result.Name != "" {
		t.Errorf("Name = %q, want empty", result.Name)
	}
}

func TestLaunchBackground_MultiByteNameRoundTrips(t *testing.T) {
	dir := t.TempDir()
	worktree := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	cwdFile := filepath.Join(dir, "cwd.txt")

	// Use printf to write unicode: "ticket with résumé"
	// We embed it directly in the script using printf with hex escapes.
	script := "#!/bin/sh\n"
	script += "printf '%s\\n' \"$@\" > '" + argvFile + "'\n"
	script += "printf '%s' \"$PWD\" > '" + cwdFile + "'\n"
	// backgrounded · cafef00d · ticket with résumé
	script += "printf 'backgrounded \\xc2\\xb7 cafef00d \\xc2\\xb7 ticket with r\\xc3\\xa9sum\\xc3\\xa9\\n'\n"
	script += "exit 0\n"
	p := filepath.Join(dir, "claude")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	result, err := LaunchBackground(context.Background(), BGArgs{
		Worktree: worktree,
		Name:     "test",
		Prompt:   "prompt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ShortID != "cafef00d" {
		t.Errorf("ShortID = %q, want cafef00d", result.ShortID)
	}
	if result.Name != "ticket with r\xc3\xa9sum\xc3\xa9" {
		t.Errorf("Name = %q, want unicode résumé", result.Name)
	}
}

// TestLaunchBackground_NegativeArgv_NoCwd is the CRITICAL regression guard.
// It verifies that --cwd is NEVER present in argv passed to the claude binary.
// cmd.Dir must be set to args.Worktree instead (verified by TestLaunchBackground_CmdDirSet).
func TestLaunchBackground_NegativeArgv_NoCwd(t *testing.T) {
	dir := t.TempDir()
	worktree := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	cwdFile := filepath.Join(dir, "cwd.txt")

	stdout := "backgrounded \xc2\xb7 abcdef12\n"
	writeFakeBGClaude(t, dir, argvFile, cwdFile, stdout, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := LaunchBackground(context.Background(), BGArgs{
		Worktree: worktree,
		Name:     "test-session",
		Prompt:   "test prompt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	argvBytes, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("could not read argv file: %v", err)
	}
	argvArgs := strings.Split(strings.TrimRight(string(argvBytes), "\n"), "\n")

	// CRITICAL ASSERTION: --cwd must never appear in argv.
	for _, arg := range argvArgs {
		if arg == "--cwd" {
			t.Errorf("REGRESSION: argv contains --cwd flag; must use cmd.Dir instead. Full argv: %v", argvArgs)
		}
	}

	// Also verify cmd.Dir was set (worktree is cwd of fake binary).
	cwdBytes, err := os.ReadFile(cwdFile)
	if err != nil {
		t.Fatalf("could not read cwd file: %v", err)
	}
	capturedCWD := string(cwdBytes)
	// On macOS /tmp is a symlink to /private/tmp; resolve both sides for comparison.
	if capturedCWD != worktree {
		// Allow /private/tmp vs /tmp divergence on macOS.
		resolvedCapture, _ := filepath.EvalSymlinks(capturedCWD)
		resolvedWorktree, _ := filepath.EvalSymlinks(worktree)
		if resolvedCapture != resolvedWorktree {
			t.Errorf("cmd.Dir not set: captured cwd = %q, want %q (or its symlink-resolved form %q)",
				capturedCWD, worktree, resolvedWorktree)
		}
	}
}

// TestLaunchBackground_CmdDirSet verifies cmd.Dir is set to args.Worktree.
func TestLaunchBackground_CmdDirSet(t *testing.T) {
	dir := t.TempDir()
	worktree := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	cwdFile := filepath.Join(dir, "cwd.txt")

	stdout := "backgrounded \xc2\xb7 99887766\n"
	writeFakeBGClaude(t, dir, argvFile, cwdFile, stdout, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := LaunchBackground(context.Background(), BGArgs{
		Worktree: worktree,
		Name:     "test",
		Prompt:   "prompt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cwdBytes, err := os.ReadFile(cwdFile)
	if err != nil {
		t.Fatalf("could not read cwd file: %v", err)
	}
	capturedCWD := string(cwdBytes)
	resolvedCapture, _ := filepath.EvalSymlinks(capturedCWD)
	resolvedWorktree, _ := filepath.EvalSymlinks(worktree)
	if resolvedCapture != resolvedWorktree {
		t.Errorf("cmd.Dir mismatch: got %q, want %q", capturedCWD, worktree)
	}
}

func TestLaunchBackground_ParseFailure(t *testing.T) {
	dir := t.TempDir()
	worktree := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	cwdFile := filepath.Join(dir, "cwd.txt")

	// Stdout has no 'backgrounded' line.
	stdout := "some other output\nnothing here\n"
	writeFakeBGClaude(t, dir, argvFile, cwdFile, stdout, 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := LaunchBackground(context.Background(), BGArgs{
		Worktree: worktree,
		Name:     "test",
		Prompt:   "prompt",
	})
	if err == nil {
		t.Fatal("expected error for missing backgrounded line, got nil")
	}
	if !strings.Contains(err.Error(), "could not parse 'backgrounded' line") {
		t.Errorf("error message %q does not contain expected text", err.Error())
	}
}

func TestLaunchBackground_ExecFailure(t *testing.T) {
	dir := t.TempDir()
	worktree := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	cwdFile := filepath.Join(dir, "cwd.txt")

	writeFakeBGClaude(t, dir, argvFile, cwdFile, "", 1)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := LaunchBackground(context.Background(), BGArgs{
		Worktree: worktree,
		Name:     "test",
		Prompt:   "prompt",
	})
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
}

func TestParseBackgroundedLine_table(t *testing.T) {
	// middle-dot literal
	dot := "\xc2\xb7"

	cases := []struct {
		name      string
		input     string
		wantID    string
		wantName  string
		wantError bool
	}{
		{
			name:     "only backgrounded line no warning",
			input:    "backgrounded " + dot + " 3174068b " + dot + " my-session\n",
			wantID:   "3174068b",
			wantName: "my-session",
		},
		{
			name:     "backgrounded preceded by one warning line",
			input:    "warning: something happened\nbackgrounded " + dot + " aabbccdd\n",
			wantID:   "aabbccdd",
			wantName: "",
		},
		{
			name:     "backgrounded preceded by three warning lines",
			input:    "warning: a\nwarning: b\nwarning: c\nbackgrounded " + dot + " 12345678 " + dot + " named\n",
			wantID:   "12345678",
			wantName: "named",
		},
		{
			name:      "no backgrounded line",
			input:     "some output\nmore output\n",
			wantError: true,
		},
		{
			name:     "trailing whitespace in name trimmed",
			input:    "backgrounded " + dot + " deadbeef " + dot + " name with spaces   \n",
			wantID:   "deadbeef",
			wantName: "name with spaces",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result, err := parseBackgroundedLine(c.input)
			if c.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ShortID != c.wantID {
				t.Errorf("ShortID = %q, want %q", result.ShortID, c.wantID)
			}
			if result.Name != c.wantName {
				t.Errorf("Name = %q, want %q", result.Name, c.wantName)
			}
		})
	}
}
