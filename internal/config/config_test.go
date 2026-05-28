package config

import (
	"encoding/json"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.SchemaVersion != 2 {
		t.Errorf("got SchemaVersion %d, want 2", cfg.SchemaVersion)
	}
	if cfg.PollIntervalSeconds != 60 {
		t.Errorf("got PollIntervalSeconds %d, want 60", cfg.PollIntervalSeconds)
	}
}

func TestConfigJSONRoundTrip(t *testing.T) {
	t.Parallel()
	orig := Config{
		SchemaVersion:       2,
		Adapter:             "jira",
		PollIntervalSeconds: 120,
		Repos: map[string]RepoEntry{
			"my-repo": {
				ID:            "my-repo",
				Name:          "My Repo",
				Path:          "/home/user/repo",
				DefaultBranch: "main",
			},
		},
		Claude: ClaudeConfig{
			Model:         "claude-opus-4-5",
			StarterPrompt: "hello",
		},
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Config
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.SchemaVersion != orig.SchemaVersion {
		t.Errorf("SchemaVersion: got %d, want %d", got.SchemaVersion, orig.SchemaVersion)
	}
	if got.Adapter != orig.Adapter {
		t.Errorf("Adapter: got %s, want %s", got.Adapter, orig.Adapter)
	}
	if got.PollIntervalSeconds != orig.PollIntervalSeconds {
		t.Errorf("PollIntervalSeconds: got %d, want %d", got.PollIntervalSeconds, orig.PollIntervalSeconds)
	}
	repo, ok := got.Repos["my-repo"]
	if !ok {
		t.Fatal("Repos: missing 'my-repo'")
	}
	if repo.Path != orig.Repos["my-repo"].Path {
		t.Errorf("Repos[my-repo].Path: got %s, want %s", repo.Path, orig.Repos["my-repo"].Path)
	}
}

func TestConfigJSONTags(t *testing.T) {
	t.Parallel()
	cfg := Config{SchemaVersion: 2, Adapter: "jira"}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal into map: %v", err)
	}
	if _, ok := raw["schema_version"]; !ok {
		t.Error("expected 'schema_version' key in JSON (snake_case)")
	}
	if _, ok := raw["adapter"]; !ok {
		t.Error("expected 'adapter' key in JSON")
	}
}
