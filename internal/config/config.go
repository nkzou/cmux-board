package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nkzou/cmux-board/internal/atomicfile"
)

// SchemaVersionCurrent is the only schema version written by this binary.
// Schema v1 is refused on read (see v1_refusal.go).
const SchemaVersionCurrent = 2

// Config is stored at ~/.config/cmux-board/config.json (mode 0644).
type Config struct {
	SchemaVersion       int                  `json:"schema_version"`
	Adapter             string               `json:"adapter"`            // "jira"
	AdapterConfig       map[string]any       `json:"adapter_config,omitempty"`
	BoardID             string               `json:"board_id,omitempty"` // tracker board ID
	Repos               map[string]RepoEntry `json:"repos,omitempty"`    // keyed by repo_id
	PollIntervalSeconds int                  `json:"poll_interval_seconds,omitempty"`
	WorktreeBaseDir     string               `json:"worktree_base_dir,omitempty"`
	Claude              ClaudeConfig         `json:"claude,omitempty"`

	// DryRun, when true, makes the binary read-only against the tracker.
	// No TransitionStatus calls are issued; the poller continues to read tickets and
	// boards normally. Card-move attempts return Result{DryRun: true} so the UI can
	// surface a non-modal toast and snap the card back to its prior column.
	// Settable via config file or the --dry-run CLI flag (wired in M-008).
	DryRun bool `json:"dry_run,omitempty"`
}

// ColumnConfig is one entry in the adapter_config.columns array.
// Statuses is a list of Jira status names (not IDs) that map to this column.
// The order of entries in the parent array is the display order on the board.
type ColumnConfig struct {
	Name     string   `json:"name"`
	Statuses []string `json:"statuses"`
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
	return atomicfile.WriteFile(path, data, 0644)
}

// ParseAdapterColumns reads the "columns" key from an adapter_config map and returns
// the parsed []ColumnConfig. Returns an empty slice (never nil) if the key is absent
// or the value cannot be decoded. Errors are non-fatal by design: a missing or
// malformed columns entry degrades gracefully to "all tickets in Unmapped".
func ParseAdapterColumns(adapterCfg map[string]any) []ColumnConfig {
	if adapterCfg == nil {
		return []ColumnConfig{}
	}
	raw, ok := adapterCfg["columns"]
	if !ok {
		return []ColumnConfig{}
	}
	// Re-marshal then unmarshal through the typed struct to handle the
	// map[string]any representation that JSON unmarshaling produces.
	data, err := json.Marshal(raw)
	if err != nil {
		return []ColumnConfig{}
	}
	var cols []ColumnConfig
	if err := json.Unmarshal(data, &cols); err != nil {
		return []ColumnConfig{}
	}
	return cols
}

// SetAdapterColumns stores cols into the "columns" key of adapterCfg.
// If adapterCfg is nil, a new map is allocated and returned.
func SetAdapterColumns(adapterCfg map[string]any, cols []ColumnConfig) map[string]any {
	if adapterCfg == nil {
		adapterCfg = make(map[string]any)
	}
	// Store as a value that will round-trip correctly through JSON.
	adapterCfg["columns"] = cols
	return adapterCfg
}
