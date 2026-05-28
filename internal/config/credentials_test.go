package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nkzou/cmux-board/internal/secretsink"
)

func TestConfigLoadSaveRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	orig := Config{
		SchemaVersion:       2,
		Adapter:             "jira",
		PollIntervalSeconds: 90,
		Repos: map[string]RepoEntry{
			"r1": {ID: "r1", Name: "Repo1", Path: "/tmp/r1", DefaultBranch: "main"},
		},
	}
	if err := Save(path, orig); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
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
	if _, ok := got.Repos["r1"]; !ok {
		t.Error("Repos['r1'] missing after round-trip")
	}
}

func TestConfigLoadMissing(t *testing.T) {
	t.Parallel()
	got, err := Load("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("expected DefaultConfig for missing file, got error: %v", err)
	}
	if got.SchemaVersion != SchemaVersionCurrent {
		t.Errorf("SchemaVersion: got %d, want %d", got.SchemaVersion, SchemaVersionCurrent)
	}
}

func TestConfigLoadCorrupt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{invalid`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for corrupt config, got nil")
	}
}

func TestCredentialsLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	orig := Credentials{
		SchemaVersion: 2,
		Adapters: map[string]AdapterCreds{
			"jira": {
				Email:    "user@example.com",
				APIToken: "testtoken12345",
				SiteURL:  "https://example.atlassian.net",
			},
		},
	}
	if err := SaveCredentials(path, orig); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	got, err := LoadCredentials(path)
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if got.Adapters["jira"].Email != orig.Adapters["jira"].Email {
		t.Errorf("Email: got %s, want %s", got.Adapters["jira"].Email, orig.Adapters["jira"].Email)
	}
	if got.Adapters["jira"].APIToken != orig.Adapters["jira"].APIToken {
		t.Errorf("APIToken: got %s, want %s", got.Adapters["jira"].APIToken, orig.Adapters["jira"].APIToken)
	}
}

func TestCredentialsLoadRegistersSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	token := "uniquetesttoken9876543210"
	creds := Credentials{
		SchemaVersion: 2,
		Adapters: map[string]AdapterCreds{
			"jira": {APIToken: token},
		},
	}
	if err := SaveCredentials(path, creds); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	_, err := LoadCredentials(path)
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	// Verify the token is now redacted
	redacted := secretsink.RedactBytes([]byte("api call with " + token + " auth"))
	for _, b := range []byte(token) {
		_ = b
	}
	// Token should NOT appear in redacted output
	if string(redacted) == "api call with "+token+" auth" {
		t.Error("token was not registered for redaction after LoadCredentials")
	}
}

func TestCredentialsFileModeAfterSave(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	creds := Credentials{SchemaVersion: 2}
	if err := SaveCredentials(path, creds); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("credentials file mode: got %04o, want 0600", info.Mode().Perm())
	}
}

func TestConfigFileModeAfterSave(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := DefaultConfig()
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("config file mode: got %04o, want 0644", info.Mode().Perm())
	}
}
