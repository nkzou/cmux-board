package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

	// SECURITY: --api-token is intentionally NOT a flag. The token is read
	// exclusively via --api-token-stdin (stdin pipe) or interactive masked prompt.
	// F4 / CONVENTIONS §"API token via --api-token-stdin or interactive only".
	initCmd.Flags().Bool("api-token-stdin", false,
		"read API token from stdin (pipe or redirect); token is never passed via flag value")

	// Additional per-field flags for non-interactive mode.
	initCmd.Flags().String("adapter", "", "adapter name (e.g. jira)")
	initCmd.Flags().String("site", "", "Jira site URL (e.g. yourorg.atlassian.net)")
	initCmd.Flags().String("email", "", "Atlassian account email (used for Basic auth)")
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
	emailFlag, _ := cmd.Flags().GetString("email")
	boardIDFlag, _ := cmd.Flags().GetString("board-id")
	repoFlag, _ := cmd.Flags().GetStringSlice("repo")
	tokenFromStdin, _ := cmd.Flags().GetBool("api-token-stdin")

	// 1. Adapter pick (v1 always picks jira; flag is accepted but the menu still shows
	// the single option for clarity).
	if _, err := initwizard.PromptAdapterPick(stdin, stdout); err != nil {
		return fmt.Errorf("adapter pick: %w", err)
	}

	// 2. Credentials (site + token).
	creds, err := initwizard.PromptJiraCredentials(stdin, stdout, siteFlag, tokenFromStdin)
	if err != nil {
		return fmt.Errorf("credentials: %w", err)
	}

	// 3. Email (Basic-auth requires email; collected separately from token since email
	// is not a secret and not registered with secretsink).
	email, err := promptEmail(stdin, stdout, emailFlag)
	if err != nil {
		return fmt.Errorf("email: %w", err)
	}

	// 4. Build the jira adapter from collected credentials.
	adapter := jira.NewJiraAdapter(jira.Credentials{
		Site:     creds.Site,
		Email:    email,
		APIToken: creds.APIToken,
	})

	// 5. Auth probe (WhoAmI) — fails fast on a bad token.
	identity, err := initwizard.ProbeAuth(ctx, stdout, adapter)
	if err != nil {
		return err
	}

	// 6. Board pick (interactive or --board-id override).
	board, err := initwizard.PickBoard(ctx, stdout, stdin, adapter, boardIDFlag)
	if err != nil {
		return fmt.Errorf("board pick: %w", err)
	}

	// 7. Repo registration (interactive loop or --repo override).
	repos, err := initwizard.RegisterRepos(ctx, stdout, stdin, repoFlag, nil, nil)
	if err != nil {
		return fmt.Errorf("repos: %w", err)
	}

	// 8. Finalize: confirmation prompt, write three files atomically, print Dock snippet.
	input := initwizard.WizardInput{
		Adapter:         "jira",
		Site:            creds.Site,
		Email:           email,
		APIToken:        creds.APIToken,
		BoardID:         board.ID,
		BoardName:       board.Name,
		WorktreeBaseDir: filepath.Join(configDir, "worktrees"),
		DefaultApproach: "main",
		Repos:           repos,
		UserID:          identity.ID,
	}
	return initwizard.Finalize(ctx, stdout, stdin, input, configDir)
}

// promptEmail collects the Atlassian account email. If hint is non-empty (from --email),
// the prompt is skipped. Empty input on hard return is rejected (no default).
func promptEmail(r io.Reader, w io.Writer, hint string) (string, error) {
	if hint != "" {
		return hint, nil
	}
	reader := bufio.NewReader(r)
	for attempt := 0; attempt < 3; attempt++ {
		fmt.Fprint(w, "Atlassian account email: ")
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return "", fmt.Errorf("reading email: %w", err)
		}
		email := strings.TrimSpace(line)
		if strings.Contains(email, "@") && strings.Contains(email, ".") {
			return email, nil
		}
		fmt.Fprintln(w, "Expected an email address (must contain '@' and '.').")
	}
	return "", fmt.Errorf("no valid email after 3 attempts")
}
