package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/kevin-zou/cmux-board/internal/config"
)

// withTempConfigDir sets configDirOverride for the duration of f and restores it.
// It also resets cobra flag state between tests to prevent bleed-through.
func withTempConfigDir(t *testing.T, f func(dir string)) {
	t.Helper()
	dir := t.TempDir()
	prev := configDirOverride
	configDirOverride = dir
	t.Cleanup(func() {
		configDirOverride = prev
		// Reset all init flags to defaults to prevent bleed-through between tests.
		initCmd.Flags().Set("force", "false")
		initCmd.Flags().Set("reset", "false")
		initCmd.Flags().Set("api-token-stdin", "false")
		initCmd.Flags().Set("non-interactive", "false")
		initCmd.Flags().Set("adapter", "")
		initCmd.Flags().Set("site", "")
		initCmd.Flags().Set("board-id", "")
	})
	f(dir)
}

// noopWizard returns a wizardFunc that immediately returns nil.
func noopWizard() func(*cobra.Command, string) error {
	return func(_ *cobra.Command, _ string) error { return nil }
}

// writeThreeFiles writes minimal valid files for testing.
func writeThreeFiles(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{config.ConfigFileName, config.StateFileName, config.CredentialsFileName} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(`{"schema_version":2}`), 0644); err != nil {
			t.Fatalf("writeThreeFiles: %v", err)
		}
	}
}

func TestInitCmd_ForceAndResetMutuallyExclusive(t *testing.T) {
	withTempConfigDir(t, func(dir string) {
		prev := wizardFunc
		wizardFunc = noopWizard()
		t.Cleanup(func() { wizardFunc = prev })

		rootCmd.SetArgs([]string{"init", "--force", "--reset"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error for --force --reset")
		}
		if !strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("expected 'mutually exclusive' in error, got: %v", err)
		}
	})
}

func TestInitCmd_ResetRemovesAllThreeFiles(t *testing.T) {
	withTempConfigDir(t, func(dir string) {
		writeThreeFiles(t, dir)

		prev := wizardFunc
		wizardFunc = noopWizard()
		t.Cleanup(func() { wizardFunc = prev })

		rootCmd.SetArgs([]string{"init", "--reset"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, name := range []string{config.ConfigFileName, config.StateFileName, config.CredentialsFileName} {
			if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
				t.Errorf("expected %s to be removed by --reset, still exists", name)
			}
		}
	})
}

func TestInitCmd_ResetToleratesMissingFiles(t *testing.T) {
	withTempConfigDir(t, func(dir string) {
		prev := wizardFunc
		wizardFunc = noopWizard()
		t.Cleanup(func() { wizardFunc = prev })

		rootCmd.SetArgs([]string{"init", "--reset"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("expected no error for missing files: %v", err)
		}
	})
}

func TestInitCmd_ForcePreservesStateJSON(t *testing.T) {
	withTempConfigDir(t, func(dir string) {
		sentinel := []byte(`{"schema_version":2,"sentinel":true}`)
		statePath := filepath.Join(dir, config.StateFileName)
		if err := os.WriteFile(statePath, sentinel, 0644); err != nil {
			t.Fatalf("setup: %v", err)
		}

		prev := wizardFunc
		wizardFunc = func(_ *cobra.Command, configDir string) error {
			// Simulate wizard writing a fresh state.json.
			newState := []byte(`{"schema_version":2,"tickets":{}}`)
			return os.WriteFile(filepath.Join(configDir, config.StateFileName), newState, 0644)
		}
		t.Cleanup(func() { wizardFunc = prev })

		rootCmd.SetArgs([]string{"init", "--force"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("state.json missing: %v", err)
		}
		var gotMap, sentinelMap map[string]interface{}
		json.Unmarshal(got, &gotMap)
		json.Unmarshal(sentinel, &sentinelMap)
		if v, ok := sentinelMap["sentinel"]; !ok || v != gotMap["sentinel"] {
			t.Errorf("state.json not preserved: got %s, want %s", got, sentinel)
		}
	})
}

func TestInitCmd_ForceNoStateInitially(t *testing.T) {
	withTempConfigDir(t, func(dir string) {
		prev := wizardFunc
		wizardFunc = func(_ *cobra.Command, configDir string) error {
			newState := []byte(`{"schema_version":2,"tickets":{}}`)
			return os.WriteFile(filepath.Join(configDir, config.StateFileName), newState, 0644)
		}
		t.Cleanup(func() { wizardFunc = prev })

		rootCmd.SetArgs([]string{"init", "--force"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(filepath.Join(dir, config.StateFileName)); os.IsNotExist(err) {
			t.Error("expected state.json to be created by wizard in --force without pre-existing file")
		}
	})
}

func TestInitCmd_NoFlags_NormalFlow(t *testing.T) {
	withTempConfigDir(t, func(dir string) {
		prev := wizardFunc
		wizardFunc = noopWizard()
		t.Cleanup(func() { wizardFunc = prev })

		rootCmd.SetArgs([]string{"init"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
