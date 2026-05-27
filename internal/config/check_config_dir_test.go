package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckConfigDir_V1ConfigRefuses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(`{"schema_version":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	err := CheckConfigDir(dir)
	if err == nil {
		t.Fatal("expected error for v1 config.json")
	}
	if !errors.Is(err, ErrSchemaV1) {
		t.Errorf("expected ErrSchemaV1, got: %v", err)
	}
	// Error must include the file path.
	if !containsString(err.Error(), ConfigFileName) {
		t.Errorf("error should mention the file path, got: %v", err)
	}
}

func TestCheckConfigDir_V1StateRefuses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, StateFileName), []byte(`{"schema_version":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	err := CheckConfigDir(dir)
	if err == nil {
		t.Fatal("expected error for v1 state.json")
	}
	if !errors.Is(err, ErrSchemaV1) {
		t.Errorf("expected ErrSchemaV1, got: %v", err)
	}
}

func TestCheckConfigDir_V2Passes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(`{"schema_version":2}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, StateFileName), []byte(`{"schema_version":2}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := CheckConfigDir(dir); err != nil {
		t.Errorf("expected nil for v2, got: %v", err)
	}
}

func TestCheckConfigDir_MissingFilesPass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No files written — fresh install.
	if err := CheckConfigDir(dir); err != nil {
		t.Errorf("expected nil for missing files, got: %v", err)
	}
}

func TestCheckConfigDir_SchemaVersion0Refuses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(`{"schema_version":0}`), 0644); err != nil {
		t.Fatal(err)
	}
	err := CheckConfigDir(dir)
	if err == nil {
		t.Fatal("expected error for schema_version:0")
	}
	if !errors.Is(err, ErrSchemaV1) {
		t.Errorf("expected ErrSchemaV1, got: %v", err)
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
