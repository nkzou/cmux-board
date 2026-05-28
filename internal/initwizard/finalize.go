package initwizard

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

// WizardInput carries all values collected by the preceding wizard steps.
type WizardInput struct {
	Adapter         string             // e.g. "jira"
	Site            string
	Email           string             // resolved from acli auth status; for display only
	BoardID         string
	BoardName       string
	WorktreeBaseDir string
	DefaultApproach string
	Repos           []config.RepoEntry
	// UserID is the resolved Jira user ID from WhoAmI (stored in adapter_config).
	UserID  string
	// Columns is the manually-configured board column layout from ConfigureColumns.
	// Stored in adapter_config.columns in config.json.
	Columns []config.ColumnConfig
}

// Finalize prints a summary, prompts for confirmation, writes all three files,
// verifies mode bits, and prints the template help.
// configDir is the directory for all three files (default ~/.config/cmux-board/).
func Finalize(ctx context.Context, w io.Writer, r io.Reader, input WizardInput, configDir string) error {
	// Print summary.
	fmt.Fprintln(w, "--- cmux-board init summary ---")
	fmt.Fprintf(w, "Adapter:       %s\n", input.Adapter)
	fmt.Fprintf(w, "Site:          %s\n", input.Site)
	if input.Email != "" {
		fmt.Fprintf(w, "Email:         %s\n", input.Email)
	}
	fmt.Fprintf(w, "Board:         %s (%s)\n", input.BoardName, input.BoardID)
	fmt.Fprintf(w, "Worktree base: %s\n", input.WorktreeBaseDir)
	fmt.Fprintf(w, "Approach:      %s\n", input.DefaultApproach)
	fmt.Fprintf(w, "Repos:         %d registered\n", len(input.Repos))
	fmt.Fprintln(w, "--------------------------------")

	// Confirmation prompt.
	reader := bufio.NewReader(r)
	fmt.Fprint(w, "Write config? [Y/n]: ")
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer == "n" {
		fmt.Fprintln(w, "Aborted — no files written.")
		return nil
	}

	// Ensure config directory exists.
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config dir %s: %w", configDir, err)
	}

	// (a) Write config.json at 0644.
	cfg := buildConfig(input)
	cfgPath := filepath.Join(configDir, config.ConfigFileName)
	cfgData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config.json: %w", err)
	}
	if err := config.WriteFileAtomic(cfgPath, cfgData, 0644); err != nil {
		return fmt.Errorf("failed to write config.json: %w", err)
	}

	// (b) Write state.json at 0644, schema v2, empty board.
	st := state.DefaultState()
	statePath := filepath.Join(configDir, config.StateFileName)
	stateData, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state.json: %w", err)
	}
	if err := config.WriteFileAtomic(statePath, stateData, 0644); err != nil {
		return fmt.Errorf("failed to write state.json: %w", err)
	}

	// (c) Write credentials.json at 0600, then verify.
	creds := buildCredentials(input)
	credsPath := filepath.Join(configDir, config.CredentialsFileName)
	credsData, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials.json: %w", err)
	}
	if err := config.WriteFileAtomic(credsPath, credsData, 0600); err != nil {
		return fmt.Errorf("failed to write credentials.json: %w", err)
	}
	if err := os.Chmod(credsPath, 0600); err != nil {
		return fmt.Errorf("failed to chmod credentials.json: %w", err)
	}
	fi, err := os.Stat(credsPath)
	if err != nil {
		return fmt.Errorf("failed to stat credentials.json: %w", err)
	}
	if fi.Mode().Perm() != 0600 {
		return fmt.Errorf("credentials.json mode is %04o after chmod; expected 0600", fi.Mode().Perm())
	}

	// (d) chmod parent dir to 0700 (best-effort, non-fatal).
	if err := os.Chmod(configDir, 0700); err != nil {
		slog.Warn("failed to chmod config dir", "dir", configDir, "err", err)
	}

	// Print template help.
	PrintTemplateHelp(w)

	return nil
}

// PrintTemplateHelp emits a short block explaining available template variables.
func PrintTemplateHelp(w io.Writer) {
	fmt.Fprintln(w, "--- Starter prompt template variables ---")
	fmt.Fprintln(w, "{{.Ticket.Key}}       Jira issue key (e.g. PROJ-42)")
	fmt.Fprintln(w, "{{.Ticket.Summary}}   Issue title")
	fmt.Fprintln(w, "{{.Ticket.Status}}    Current status name")
	fmt.Fprintln(w, "{{.Ticket.URL}}       Browser link to the issue")
	fmt.Fprintln(w, "{{.Repo.Name}}        Display name of the registered repo")
	fmt.Fprintln(w, "{{.Repo.Path}}        Absolute path to the git clone")
	fmt.Fprintln(w, "{{.WorktreePath}}     Absolute path to the created worktree")
	fmt.Fprintln(w, "{{.ApproachName}}     Approach name (e.g. \"main\")")
	fmt.Fprintln(w, "-----------------------------------------")
	fmt.Fprintln(w, "Example: edit config.json → \"claude.starter_prompt_template\"")
}

// buildConfig constructs a config.Config from the wizard input.
func buildConfig(input WizardInput) config.Config {
	cfg := config.DefaultConfig()
	cfg.Adapter = input.Adapter
	cfg.AdapterConfig = map[string]any{
		"site":    input.Site,
		"email":   input.Email,
		"user_id": input.UserID,
	}
	cfg.BoardID = input.BoardID
	cfg.WorktreeBaseDir = input.WorktreeBaseDir
	cfg.Claude = config.ClaudeConfig{}
	if input.DefaultApproach != "" {
		cfg.AdapterConfig["default_approach_name"] = input.DefaultApproach
	}
	// Persist manually-configured columns into adapter_config.columns.
	if len(input.Columns) > 0 {
		cfg.AdapterConfig = config.SetAdapterColumns(cfg.AdapterConfig, input.Columns)
	}
	// Populate repos map.
	if len(input.Repos) > 0 {
		cfg.Repos = make(map[string]config.RepoEntry, len(input.Repos))
		for _, r := range input.Repos {
			cfg.Repos[r.ID] = r
		}
	}
	return cfg
}

// credentialsSchema is the schema_version for credentials.json.
// Intentionally pinned at v1 per persistence spec; only config.json and state.json are v2.
const credentialsSchema = 1

// credentialsFile is the on-disk representation of credentials.json.
// Uses schema_version 1 (intentionally pinned).
type credentialsFile struct {
	SchemaVersion int                     `json:"schema_version"`
	Adapters      map[string]adapterCreds `json:"adapters,omitempty"`
}

type adapterCreds struct {
	AuthMethod string `json:"auth_method,omitempty"`
	SiteURL    string `json:"site_url,omitempty"`
}

// buildCredentials constructs the credentials file content from wizard input.
// For the acli-backed jira adapter, no secrets are stored here — acli owns auth.
// The file is kept for the 0600 mode-bit validation pattern.
func buildCredentials(input WizardInput) credentialsFile {
	return credentialsFile{
		SchemaVersion: credentialsSchema,
		Adapters: map[string]adapterCreds{
			"jira": {
				AuthMethod: "acli",
				SiteURL:    input.Site,
			},
		},
	}
}
