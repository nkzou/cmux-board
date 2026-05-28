package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateCredentialsModeOK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	f.Close()
	if err := ValidateCredentialsMode(path, false); err != nil {
		t.Errorf("expected nil error for mode 0600, got: %v", err)
	}
}

func TestValidateCredentialsModeUnsafe(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	f.Close()
	if err := ValidateCredentialsMode(path, false); err == nil {
		t.Error("expected error for mode 0644, got nil")
	}
}

func TestValidateCredentialsModeUnsafeFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	f.Close()
	// With unsafe=true, should not error even though mode is 0644
	if err := ValidateCredentialsMode(path, true); err != nil {
		t.Errorf("expected nil with unsafe=true, got: %v", err)
	}
}

func TestValidateCredentialsModeNotExist(t *testing.T) {
	t.Parallel()
	if err := ValidateCredentialsMode("/nonexistent/credentials.json", false); err != nil {
		t.Errorf("expected nil for non-existent file, got: %v", err)
	}
}

func TestEnsureConfigDirModeOK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Set to 0700
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if err := EnsureConfigDirMode(dir); err != nil {
		t.Errorf("expected nil for mode 0700, got: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("mode changed unexpectedly: got %04o, want 0700", info.Mode().Perm())
	}
}

func TestEnsureConfigDirModeTightens(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Set to 0755 (too loose)
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatalf("chmod to 0755: %v", err)
	}
	if err := EnsureConfigDirMode(dir); err != nil {
		t.Errorf("EnsureConfigDirMode: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("expected mode 0700 after tightening, got %04o", info.Mode().Perm())
	}
}
