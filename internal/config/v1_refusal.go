package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
