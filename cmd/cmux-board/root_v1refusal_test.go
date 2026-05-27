package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevin-zou/cmux-board/internal/config"
)

// setConfigDirEnv sets CMUX_BOARD_CONFIG_DIR for the duration of the test.
func setConfigDirEnv(t *testing.T, dir string) {
	t.Helper()
	t.Setenv(config.EnvConfigDir, dir)
}

func TestRootV1Refusal_V1ConfigRefusesAllSubcommands(t *testing.T) {
	dir := t.TempDir()
	setConfigDirEnv(t, dir)
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(`{"schema_version":1}`), 0644); err != nil {
		t.Fatal(err)
	}

	// dock subcommand should be refused.
	rootCmd.SetArgs([]string{"dock"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected schema v1 error for dock command")
	}
	if !errors.Is(err, config.ErrSchemaV1) && !strings.Contains(err.Error(), "schema v1 detected") {
		t.Errorf("expected 'schema v1 detected', got: %v", err)
	}
}

func TestRootV1Refusal_V1StateRefusesAllSubcommands(t *testing.T) {
	dir := t.TempDir()
	setConfigDirEnv(t, dir)
	if err := os.WriteFile(filepath.Join(dir, config.StateFileName), []byte(`{"schema_version":1}`), 0644); err != nil {
		t.Fatal(err)
	}

	rootCmd.SetArgs([]string{"repos", "list"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected schema v1 error for repos list command")
	}
	if !errors.Is(err, config.ErrSchemaV1) && !strings.Contains(err.Error(), "schema v1 detected") {
		t.Errorf("expected 'schema v1 detected', got: %v", err)
	}
}

func TestRootV1Refusal_V2Allowed(t *testing.T) {
	dir := t.TempDir()
	setConfigDirEnv(t, dir)
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(`{"schema_version":2}`), 0644); err != nil {
		t.Fatal(err)
	}
	// PersistentPreRunE passes — CheckConfigDir returns nil.
	err := config.CheckConfigDir(dir)
	if err != nil {
		t.Errorf("expected nil for v2, got: %v", err)
	}
}

func TestRootV1Refusal_MissingFilesAllowed(t *testing.T) {
	dir := t.TempDir()
	// No files exist.
	err := config.CheckConfigDir(dir)
	if err != nil {
		t.Errorf("expected nil for missing files, got: %v", err)
	}
}

func TestRootV1Refusal_SchemaVersion0Refused(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(`{"schema_version":0}`), 0644); err != nil {
		t.Fatal(err)
	}
	err := config.CheckConfigDir(dir)
	if err == nil {
		t.Fatal("expected error for schema_version:0")
	}
	if !errors.Is(err, config.ErrSchemaV1) {
		t.Errorf("expected ErrSchemaV1, got: %v", err)
	}
}

func TestRootV1Refusal_InitSubcommandExempt(t *testing.T) {
	// "init" is exempt from the v1 check — it should run even with a v1 config.
	dir := t.TempDir()
	setConfigDirEnv(t, dir)
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(`{"schema_version":1}`), 0644); err != nil {
		t.Fatal(err)
	}

	prev := wizardFunc
	wizardFunc = noopWizard()
	t.Cleanup(func() { wizardFunc = prev })

	prev2 := configDirOverride
	configDirOverride = dir
	t.Cleanup(func() {
		configDirOverride = prev2
		initCmd.Flags().Set("force", "false")
		initCmd.Flags().Set("reset", "false")
	})

	rootCmd.SetArgs([]string{"init"})
	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("init should be exempt from v1 refusal, got: %v", err)
	}
}
