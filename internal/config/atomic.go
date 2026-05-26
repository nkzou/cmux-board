package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// renameFunc is the function used to rename files atomically.
// Overridable in tests for fault injection.
var renameFunc = os.Rename

// WriteFileAtomic writes data to path atomically using a sibling temp file + os.Rename.
// If path does not exist it is created. perm is applied to the temp file before rename.
// Parent directory must already exist. On macOS, parent dir fsync is best-effort (ignored).
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	f, err := os.CreateTemp(dir, base+".tmp.*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for %s: %w", path, err)
	}
	tmpPath := f.Name()
	// On error: clean up temp file
	defer func() {
		if f != nil {
			f.Close()
			os.Remove(tmpPath)
		}
	}()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}
	if err := f.Chmod(perm); err != nil {
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to fsync temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	f = nil // prevent defer from removing

	// Best-effort parent dir fsync (macOS journal entry)
	if df, err := os.Open(dir); err == nil {
		_ = df.Sync() // ignore error on macOS (not supported by some filesystems)
		df.Close()
	}

	if err := renameFunc(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file to %s: %w", path, err)
	}
	return nil
}
