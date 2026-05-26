package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckSchemaVersionV2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":2}`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := CheckSchemaVersion(path); err != nil {
		t.Errorf("expected nil for v2, got: %v", err)
	}
}

func TestCheckSchemaVersionV1(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1}`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	err := CheckSchemaVersion(path)
	if err == nil {
		t.Fatal("expected ErrSchemaV1, got nil")
	}
	if !errors.Is(err, ErrSchemaV1) {
		t.Errorf("expected errors.Is(err, ErrSchemaV1), got: %v", err)
	}
}

func TestCheckSchemaVersionMissing(t *testing.T) {
	t.Parallel()
	if err := CheckSchemaVersion("/nonexistent/config.json"); err != nil {
		t.Errorf("expected nil for missing file, got: %v", err)
	}
}

func TestCheckSchemaVersionCorrupt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{invalid json`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	err := CheckSchemaVersion(path)
	if err == nil {
		t.Fatal("expected error for corrupt JSON, got nil")
	}
	if errors.Is(err, ErrSchemaV1) {
		t.Error("corrupt JSON should not return ErrSchemaV1")
	}
}

func TestCheckSchemaVersionZero(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":0}`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := CheckSchemaVersion(path); err != nil {
		t.Errorf("expected nil for schema_version 0, got: %v", err)
	}
}
