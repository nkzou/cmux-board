package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	content := []byte(`{"hello":"world"}`)
	if err := WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
	// Check file mode
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("file mode: got %04o, want 0644", info.Mode().Perm())
	}
}

func TestWriteFileCreatesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "newfile.json")
	content := []byte(`{"created":true}`)
	if err := WriteFile(path, content, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestWriteFileRenameFailure(t *testing.T) {
	// Do NOT call t.Parallel(): this test mutates a package-level var (renameFunc).
	dir := t.TempDir()
	path := filepath.Join(dir, "target.json")
	original := []byte(`{"original":true}`)
	// Write initial content
	if err := WriteFile(path, original, 0644); err != nil {
		t.Fatalf("initial WriteFile: %v", err)
	}

	// Override renameFunc to simulate failure
	oldRename := renameFunc
	renameFunc = func(_, _ string) error {
		return errors.New("simulated rename failure")
	}
	defer func() { renameFunc = oldRename }()

	// Attempt to overwrite with new content — should fail
	if err := WriteFile(path, []byte(`{"new":true}`), 0644); err == nil {
		t.Fatal("expected error from WriteFile on rename failure, got nil")
	}

	// Original content must still be intact
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after failure: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("original content corrupted: got %q, want %q", got, original)
	}

	// No .tmp.* files left behind
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}
