package claudecli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMeetsMin(t *testing.T) {
	cases := []struct {
		actual string
		min    string
		want   bool
	}{
		{"2.1.150", "2.1.150", true},  // equal to min
		{"2.1.151", "2.1.150", true},  // patch newer
		{"2.2.0", "2.1.150", true},    // minor newer
		{"3.0.0", "2.1.150", true},    // major newer
		{"2.1.149", "2.1.150", false}, // one patch behind
		{"2.0.999", "2.1.150", false}, // minor behind
		{"1.9.999", "2.1.150", false}, // major behind
	}
	for _, c := range cases {
		got := MeetsMin(c.actual, c.min)
		if got != c.want {
			t.Errorf("MeetsMin(%q, %q) = %v, want %v", c.actual, c.min, got, c.want)
		}
	}
}

// writeFakeClaude writes a shell script acting as a fake `claude` binary into dir,
// named "claude", and returns dir. The script prints the given output to stdout and
// exits with the given code.
func writeFakeClaude(t *testing.T, dir, stdout string, exitCode int) {
	t.Helper()
	script := "#!/bin/sh\n"
	if exitCode != 0 {
		script += "echo 'error: fake failure' >&2\n"
		script += "exit " + string(rune('0'+exitCode)) + "\n"
	} else {
		script += "printf '%s' " + "'" + stdout + "'" + "\n"
		script += "exit 0\n"
	}
	p := filepath.Join(dir, "claude")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("writeFakeClaude: %v", err)
	}
}

func TestVersion_ParsesStandardFormat(t *testing.T) {
	dir := t.TempDir()
	writeFakeClaude(t, dir, "2.1.150 (Claude Code)\n", 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	v, err := Version(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "2.1.150" {
		t.Errorf("got %q, want %q", v, "2.1.150")
	}
}

func TestVersion_ParsesAlternateFormat(t *testing.T) {
	dir := t.TempDir()
	writeFakeClaude(t, dir, "claude 2.1.150\n", 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	v, err := Version(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "2.1.150" {
		t.Errorf("got %q, want %q", v, "2.1.150")
	}
}

func TestVersion_GarbageOutput(t *testing.T) {
	dir := t.TempDir()
	writeFakeClaude(t, dir, "not-a-version\n", 0)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := Version(context.Background())
	if err == nil {
		t.Fatal("expected error for garbage output, got nil")
	}
}

func TestVersion_ExecFailure(t *testing.T) {
	dir := t.TempDir()
	writeFakeClaude(t, dir, "", 1)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := Version(context.Background())
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
}
