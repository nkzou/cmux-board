package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/initwizard"
	"github.com/nkzou/cmux-board/internal/tracker/jira"
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

	// Per-field flags for non-interactive mode.
	initCmd.Flags().String("adapter", "", "adapter name (e.g. jira)")
	initCmd.Flags().String("site", "", "Jira site URL (e.g. yourorg.atlassian.net); resolved from acli auth status if not provided")
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
// Test harness (T-075) overrides via wizardFunc.
func runInitWizard(cmd *cobra.Command, configDir string) error {
	return wizardFunc(cmd, configDir)
}

// wizardFunc is the injectable wizard implementation used in production and tests.
// Tests override this to a stub that immediately returns nil (simulating user confirm).
var wizardFunc = defaultWizardFunc

func defaultWizardFunc(cmd *cobra.Command, configDir string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	stdout := cmd.OutOrStdout()
	stdin := cmd.InOrStdin()

	siteFlag, _ := cmd.Flags().GetString("site")
	boardIDFlag, _ := cmd.Flags().GetString("board-id")
	repoFlag, _ := cmd.Flags().GetStringSlice("repo")

	// 1. Adapter pick (v1 always picks jira; flag is accepted but the menu still shows
	// the single option for clarity).
	if _, err := initwizard.PromptAdapterPick(stdin, stdout); err != nil {
		return fmt.Errorf("adapter pick: %w", err)
	}

	// 2. Build adapter — site from flag or resolved from acli auth status.
	// ProbeACLI will determine the site by calling WhoAmI (acli auth status).
	adapter := jira.NewJiraAdapter(jira.Config{
		Site: siteFlag,
	})

	// 3. Probe acli: verify installed + authenticated, resolve identity.
	identity, err := initwizard.ProbeACLI(ctx, stdout, adapter)
	if err != nil {
		return err
	}

	// If site was not provided via flag, use the site from acli auth status (identity.ID).
	site := siteFlag
	if site == "" {
		site = identity.ID // WhoAmI stores site in ID field for acli adapter
	}

	// 4. Board pick (interactive or --board-id override).
	board, err := initwizard.PickBoard(ctx, stdout, stdin, adapter, boardIDFlag)
	if err != nil {
		return fmt.Errorf("board pick: %w", err)
	}

	// 5. Column configuration: discover statuses and let the user define columns.
	trackerCols, err := initwizard.ConfigureColumns(ctx, stdout, stdin, adapter, board.ID)
	if err != nil {
		return fmt.Errorf("column configuration: %w", err)
	}
	columns := make([]config.ColumnConfig, len(trackerCols))
	for i, c := range trackerCols {
		columns[i] = config.ColumnConfig{
			Name:     c.Name,
			Statuses: c.StatusIDs,
		}
	}

	// 6. Repo registration (interactive loop or --repo override).
	repos, err := initwizard.RegisterRepos(ctx, stdout, stdin, repoFlag, nil, nil)
	if err != nil {
		return fmt.Errorf("repos: %w", err)
	}

	// 7. Finalize: confirmation prompt, write three files atomically, print Dock snippet.
	input := initwizard.WizardInput{
		Adapter:         "jira",
		Site:            site,
		Email:           identity.Email,
		BoardID:         board.ID,
		BoardName:       board.Name,
		WorktreeBaseDir: filepath.Join(configDir, "worktrees"),
		DefaultApproach: "main",
		Repos:           repos,
		UserID:          identity.ID,
		Columns:         columns,
	}
	return initwizard.Finalize(ctx, stdout, stdin, input, configDir)
}
