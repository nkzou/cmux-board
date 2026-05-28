package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrSchemaV1 is returned when a config or state file has schema_version: 1.
var ErrSchemaV1 = errors.New("schema v1 detected")

// schemaVersionOnly is used to decode just the schema_version field.
type schemaVersionOnly struct {
	SchemaVersion int `json:"schema_version"`
}

// CheckSchemaVersion reads path and returns ErrSchemaV1 if schema_version == 1.
// If path does not exist, returns nil (first-run case).
// If schema_version is missing or 0 (default), treats as current and returns nil.
func CheckSchemaVersion(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read file for schema check: %w", err)
	}
	var sv schemaVersionOnly
	if err := json.Unmarshal(data, &sv); err != nil {
		return fmt.Errorf("failed to parse schema_version from %s: %w", path, err)
	}
	if sv.SchemaVersion == 1 {
		return fmt.Errorf("%w: %s\n"+
			"Config schema v1 detected. Run `cmux-board init --migrate` to upgrade — "+
			"not implemented in v1; hand-edit since you are the only user.\n"+
			"Specifically: add \"schema_version\": 2 and the new 'repos' field to config.json; "+
			"add 'assigned_repo_ids' to each ticket in state.json.",
			ErrSchemaV1, path)
	}
	return nil
}

// CheckConfigDir checks both config.json and state.json in configDir.
// Returns ErrSchemaV1 (wrapped) if either file has schema_version < 2.
// schema_version 0 is treated as "pre-v2" and triggers a refusal because
// it indicates an uninitialized or corrupted config that was not written by
// this binary (which always writes schema_version: 2).
// Missing files are silently skipped (fresh install).
func CheckConfigDir(configDir string) error {
	for _, name := range []string{ConfigFileName, StateFileName} {
		path := filepath.Join(configDir, name)
		if err := checkFileSchemaV2(path); err != nil {
			return err
		}
	}
	return nil
}

// checkFileSchemaV2 returns ErrSchemaV1 if the file exists and has schema_version < 2.
// Missing files return nil.
func checkFileSchemaV2(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read file for schema check: %w", err)
	}
	var sv schemaVersionOnly
	if err := json.Unmarshal(data, &sv); err != nil {
		return fmt.Errorf("failed to parse schema_version from %s: %w", path, err)
	}
	if sv.SchemaVersion < 2 {
		return fmt.Errorf("%w: %s\n"+
			"Run `cmux-board init --migrate` to upgrade — "+
			"not implemented in v1; hand-edit since you are the only user.\n"+
			"(Hint: bump \"schema_version\" to 2 and add any missing fields from the v2 schema.)",
			ErrSchemaV1, path)
	}
	return nil
}
