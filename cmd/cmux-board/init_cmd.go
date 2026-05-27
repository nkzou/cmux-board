package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kevin-zou/cmux-board/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize cmux-board configuration",
	Long:  "Interactive wizard to configure the Jira adapter, register repos, and write config files.",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().Bool("force", false, "overwrite config + credentials, preserve state.json")
	initCmd.Flags().Bool("reset", false, "remove config.json, state.json, credentials.json and re-run wizard")

	// SECURITY: --api-token is intentionally NOT a flag. The token is read
	// exclusively via --api-token-stdin (stdin pipe) or interactive masked prompt.
	// F4 / CONVENTIONS §"API token via --api-token-stdin or interactive only".
	initCmd.Flags().Bool("api-token-stdin", false,
		"read API token from stdin (pipe or redirect); token is never passed via flag value")

	// Additional per-field flags for non-interactive mode.
	initCmd.Flags().String("adapter", "", "adapter name (e.g. jira)")
	initCmd.Flags().String("site", "", "Jira site URL (e.g. yourorg.atlassian.net)")
	initCmd.Flags().String("board-id", "", "board ID to select without prompting")
	initCmd.Flags().StringSlice("repo", nil, "repo paths to register non-interactively (comma-separated or repeated)")
	initCmd.Flags().Bool("non-interactive", false, "run wizard without any prompts (all values from flags)")
}

// runInit is the main RunE for the init command. Exported as a variable so tests
// can override configDirOverride for isolation.
var configDirOverride string

func runInit(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	reset, _ := cmd.Flags().GetBool("reset")

	if force && reset {
		return fmt.Errorf("--force and --reset are mutually exclusive")
	}

	configDir, err := resolveInitConfigDir()
	if err != nil {
		return err
	}

	if reset {
		if err := resetConfigFiles(configDir); err != nil {
			return err
		}
	}

	// --force: read current state.json before wizard runs (if it exists).
	var savedStateBytes []byte
	if force {
		savedStateBytes, _ = os.ReadFile(filepath.Join(configDir, config.StateFileName))
		// Ignore not-exist errors; savedStateBytes remains nil for first-run case.
	}

	// Run the wizard.
	if err := runInitWizard(cmd, configDir); err != nil {
		return err
	}

	// --force: restore state.json if it existed before the run.
	if force && savedStateBytes != nil {
		statePath := filepath.Join(configDir, config.StateFileName)
		if err := config.WriteFileAtomic(statePath, savedStateBytes, 0644); err != nil {
			return fmt.Errorf("--force: failed to restore state.json: %w", err)
		}
	}

	return nil
}

// resolveInitConfigDir returns the config dir, preferring configDirOverride (for tests),
// then CMUX_BOARD_CONFIG_DIR, then the default.
func resolveInitConfigDir() (string, error) {
	if configDirOverride != "" {
		return configDirOverride, nil
	}
	if env := os.Getenv(config.EnvConfigDir); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(home, config.DefaultConfigDir), nil
}

// resetConfigFiles removes config.json, state.json, and credentials.json from configDir.
// Ignores not-exist errors; returns any other error.
func resetConfigFiles(configDir string) error {
	for _, name := range []string{
		config.ConfigFileName,
		config.StateFileName,
		config.CredentialsFileName,
	} {
		path := filepath.Join(configDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("reset: failed to remove %s: %w", name, err)
		}
	}
	fmt.Println("Reset: all config files removed.")
	return nil
}

// runInitWizard runs the interactive (or non-interactive) init wizard.
// This is a thin shim — the real logic is in internal/initwizard.
// In non-interactive or flag-override mode, appropriate flags are read here and
// passed as hints to each wizard step.
func runInitWizard(cmd *cobra.Command, configDir string) error {
	// For now, emit a placeholder until the full wizard wiring lands in T-077.
	// The test harness in T-075 mocks this via wizardFunc.
	return wizardFunc(cmd, configDir)
}

// wizardFunc is the injectable wizard implementation used in production and tests.
// Tests override this to a stub that immediately returns nil (simulating user confirm).
var wizardFunc = defaultWizardFunc

func defaultWizardFunc(cmd *cobra.Command, configDir string) error {
	fmt.Println("cmux-board init wizard — full implementation wired in T-077/T-078")
	return nil
}
