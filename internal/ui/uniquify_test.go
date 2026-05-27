package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestUniquify_CleanPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent")
	got, sfx, err := Uniquify(path)
	if err != nil {
		t.Fatalf("Uniquify(%q) error: %v", path, err)
	}
	if got != path {
		t.Errorf("got %q, want %q", got, path)
	}
	if sfx != "" {
		t.Errorf("suffix got %q, want %q", sfx, "")
	}
}

func TestUniquify_FirstSlotTaken(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "work")
	if err := os.Mkdir(base, 0755); err != nil {
		t.Fatal(err)
	}
	got, sfx, err := Uniquify(base)
	if err != nil {
		t.Fatalf("Uniquify error: %v", err)
	}
	want := base + "-2"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if sfx != "-2" {
		t.Errorf("suffix got %q, want -2", sfx)
	}
}

func TestUniquify_TwoSlotsTaken(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "work")
	for _, p := range []string{base, base + "-2"} {
		if err := os.Mkdir(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	got, sfx, err := Uniquify(base)
	if err != nil {
		t.Fatalf("Uniquify error: %v", err)
	}
	want := base + "-3"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if sfx != "-3" {
		t.Errorf("suffix got %q, want -3", sfx)
	}
}

func TestUniquify_ThreeSlotsTaken(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "work")
	for _, p := range []string{base, base + "-2", base + "-3"} {
		if err := os.Mkdir(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	got, sfx, err := Uniquify(base)
	if err != nil {
		t.Fatalf("Uniquify error: %v", err)
	}
	want := base + "-4"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if sfx != "-4" {
		t.Errorf("suffix got %q, want -4", sfx)
	}
}

func TestUniquify_AllSlotsTaken(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "work")
	// Create base + "-2" through "-99" (and base itself).
	if err := os.Mkdir(base, 0755); err != nil {
		t.Fatal(err)
	}
	for n := 2; n <= 99; n++ {
		p := filepath.Join(dir, "work") + "-" + itoa(n)
		if err := os.Mkdir(p, 0755); err != nil {
			t.Fatalf("mkdir %q: %v", p, err)
		}
	}
	got, sfx, err := Uniquify(base)
	if !errors.Is(err, ErrUniquifyExhausted) {
		t.Errorf("error got %v, want ErrUniquifyExhausted", err)
	}
	if got != "" {
		t.Errorf("resultPath got %q, want empty", got)
	}
	if sfx != "" {
		t.Errorf("suffix got %q, want empty", sfx)
	}
}

func TestUniquify_SuffixContentsUnchanged_Sentinel(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "work")
	if err := os.Mkdir(base, 0755); err != nil {
		t.Fatal(err)
	}
	// Write a sentinel file inside the existing directory.
	sentinelPath := filepath.Join(base, "probe.txt")
	sentinelData := []byte("sentinel-contents-abc123")
	if err := os.WriteFile(sentinelPath, sentinelData, 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := Uniquify(base)
	if err != nil {
		t.Fatalf("Uniquify error: %v", err)
	}

	// Verify sentinel file is byte-equal after the call.
	got, err := os.ReadFile(sentinelPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(sentinelData) {
		t.Errorf("sentinel modified: got %q, want %q", got, sentinelData)
	}
}

func TestUniquify_TrailingSlash(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "work")
	if err := os.Mkdir(base, 0755); err != nil {
		t.Fatal(err)
	}
	// Pass path with trailing slash.
	got, sfx, err := Uniquify(base + "/")
	if err != nil {
		t.Fatalf("Uniquify error: %v", err)
	}
	want := base + "-2/"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if sfx != "-2" {
		t.Errorf("suffix got %q, want -2", sfx)
	}
}

// itoa is a minimal int-to-string helper to avoid importing strconv in test.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
