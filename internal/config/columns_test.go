package config

import (
	"encoding/json"
	"testing"
)

func TestParseAdapterColumns_HappyPath(t *testing.T) {
	t.Parallel()
	cols := []ColumnConfig{
		{Name: "To Do", Statuses: []string{"To Do", "Open"}},
		{Name: "In Progress", Statuses: []string{"In Progress", "In Review"}},
		{Name: "Done", Statuses: []string{"Done", "Closed"}},
	}
	// Simulate the map[string]any storage that JSON unmarshal produces.
	raw, _ := json.Marshal(cols)
	var asAny any
	_ = json.Unmarshal(raw, &asAny)

	adapterCfg := map[string]any{
		"site":    "example.atlassian.net",
		"columns": asAny,
	}

	got := ParseAdapterColumns(adapterCfg)
	if len(got) != 3 {
		t.Fatalf("want 3 columns, got %d", len(got))
	}
	if got[0].Name != "To Do" {
		t.Errorf("col[0].Name: got %q, want %q", got[0].Name, "To Do")
	}
	if len(got[0].Statuses) != 2 {
		t.Errorf("col[0].Statuses: got %v, want 2 entries", got[0].Statuses)
	}
	if got[2].Name != "Done" {
		t.Errorf("col[2].Name: got %q, want %q", got[2].Name, "Done")
	}
}

func TestParseAdapterColumns_MissingKey(t *testing.T) {
	t.Parallel()
	got := ParseAdapterColumns(map[string]any{"site": "example.atlassian.net"})
	if len(got) != 0 {
		t.Errorf("want empty slice for missing key, got %v", got)
	}
}

func TestParseAdapterColumns_NilMap(t *testing.T) {
	t.Parallel()
	got := ParseAdapterColumns(nil)
	if got == nil {
		t.Error("want non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("want 0 columns, got %d", len(got))
	}
}

func TestSetAndParseRoundTrip(t *testing.T) {
	t.Parallel()
	cols := []ColumnConfig{
		{Name: "Backlog", Statuses: []string{"Open", "Reopened"}},
		{Name: "In Progress", Statuses: []string{"In Progress"}},
	}

	adapterCfg := SetAdapterColumns(nil, cols)
	got := ParseAdapterColumns(adapterCfg)

	if len(got) != 2 {
		t.Fatalf("want 2 columns after round-trip, got %d", len(got))
	}
	if got[0].Name != "Backlog" {
		t.Errorf("col[0].Name: got %q, want %q", got[0].Name, "Backlog")
	}
	if len(got[0].Statuses) != 2 {
		t.Errorf("col[0].Statuses: got %v, want [Open Reopened]", got[0].Statuses)
	}
}

func TestSetAdapterColumns_PreservesExistingKeys(t *testing.T) {
	t.Parallel()
	adapterCfg := map[string]any{"site": "example.atlassian.net", "user_id": "abc123"}
	cols := []ColumnConfig{{Name: "Done", Statuses: []string{"Done"}}}

	adapterCfg = SetAdapterColumns(adapterCfg, cols)

	if adapterCfg["site"] != "example.atlassian.net" {
		t.Errorf("site key was clobbered: %v", adapterCfg["site"])
	}
	if adapterCfg["user_id"] != "abc123" {
		t.Errorf("user_id key was clobbered: %v", adapterCfg["user_id"])
	}
	got := ParseAdapterColumns(adapterCfg)
	if len(got) != 1 || got[0].Name != "Done" {
		t.Errorf("columns not set correctly: %v", got)
	}
}

func TestConfigJSONColumnsRoundTrip(t *testing.T) {
	t.Parallel()
	// Simulate a full config.json round-trip with columns in adapter_config.
	cols := []ColumnConfig{
		{Name: "To Do", Statuses: []string{"Open"}},
		{Name: "In Progress", Statuses: []string{"In Progress"}},
	}
	cfg := DefaultConfig()
	cfg.Adapter = "jira"
	cfg.AdapterConfig = SetAdapterColumns(map[string]any{"site": "ex.atlassian.net"}, cols)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var cfg2 Config
	if err := json.Unmarshal(data, &cfg2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := ParseAdapterColumns(cfg2.AdapterConfig)
	if len(got) != 2 {
		t.Fatalf("want 2 columns after config round-trip, got %d: %s", len(got), string(data))
	}
	if got[1].Name != "In Progress" {
		t.Errorf("col[1].Name: got %q, want %q", got[1].Name, "In Progress")
	}
}
