package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// SchemaVersionCurrent is the only schema version written by this binary.
// Schema v1 is refused on read (see v1_refusal.go).
const SchemaVersionCurrent = 2

// Config is stored at ~/.config/cmux-board/config.json (mode 0644).
type Config struct {
	SchemaVersion       int                `json:"schema_version"`
	Adapter             string             `json:"adapter"`                       // "jira"
	AdapterConfig       map[string]any     `json:"adapter_config,omitempty"`
	Repos               map[string]RepoEntry `json:"repos,omitempty"`             // keyed by repo_id
	PollIntervalSeconds int                `json:"poll_interval_seconds,omitempty"`
	WorktreeBaseDir     string             `json:"worktree_base_dir,omitempty"`
	Claude              ClaudeConfig       `json:"claude,omitempty"`
}

// RepoEntry is one registered repository.
type RepoEntry struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`           // absolute local path
	DefaultBranch string `json:"default_branch"`
}

// ClaudeConfig holds claude-specific launch settings.
type ClaudeConfig struct {
	Model          string `json:"model,omitempty"`
	PermissionMode string `json:"permission_mode,omitempty"`
	StarterPrompt  string `json:"starter_prompt,omitempty"`
}

// DefaultConfig returns a Config with SchemaVersion 2 and sensible defaults.
func DefaultConfig() Config {
	return Config{
		SchemaVersion:       SchemaVersionCurrent,
		PollIntervalSeconds: 60,
	}
}

// Load reads config.json from path. Returns ErrSchemaV1 if v1 detected.
func Load(path string) (Config, error) {
	if err := CheckSchemaVersion(path); err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return Config{}, fmt.Errorf("failed to read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config: %w", err)
	}
	return cfg, nil
}

// Save writes cfg to path atomically with mode 0644.
func Save(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return WriteFileAtomic(path, data, 0644)
}
