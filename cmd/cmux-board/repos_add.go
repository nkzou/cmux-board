package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nkzou/cmux-board/internal/config"
)

func newReposAddCmd() *cobra.Command {
	var (
		flagID            string
		flagName          string
		flagDefaultBranch string
	)

	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Register a git repository for ticket activation",
		Long: `Register a git repository so tickets can be activated against it.

The repository path must exist and be a real git repository. The default branch
must resolve via 'git rev-parse --verify'. If --id is omitted, the repo_id is
derived by lowercasing the display name and collapsing non-[a-z0-9-] runs to '-'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			absPath, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("failed to resolve path %q: %w", args[0], err)
			}

			// Load existing config. Refuses at schema v1.
			cfgPath, err := config.DefaultConfigPath()
			if err != nil {
				return fmt.Errorf("failed to resolve config path: %w", err)
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Validate git repo (T-047). This check MUST happen before any mutation.
			if err := config.ValidateRepoPath(absPath); err != nil {
				return err
			}

			// Derive display name from dir basename if not supplied.
			name := flagName
			if name == "" {
				name = filepath.Base(absPath)
			}

			// Derive default branch: resolve via git symbolic-ref if not supplied.
			branch := flagDefaultBranch
			if branch == "" {
				branch, err = config.ResolveDefaultBranch(absPath)
				if err != nil {
					return fmt.Errorf("failed to resolve default branch (pass --default-branch explicitly): %w", err)
				}
			}

			// Validate that the branch ref resolves (T-047).
			if err := config.ValidateDefaultBranch(absPath, branch); err != nil {
				return err
			}

			// Derive repo_id (T-048).
			id := flagID
			if id == "" {
				id = config.DeriveRepoID(name)
			}
			if id == "" {
				return fmt.Errorf("could not derive a non-empty repo_id from name %q; pass --id explicitly", name)
			}

			// Mutate in-memory config (T-049). No disk write yet.
			if err := config.AddRepo(&cfg, config.RepoArgs{
				ID:            id,
				Name:          name,
				Path:          absPath,
				DefaultBranch: branch,
			}); err != nil {
				return err
			}

			// Atomic config write (T-010 / T-015).
			if err := config.Save(cfgPath, cfg); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added: %-20s %-30s %s (%s)\n", id, name, absPath, branch)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagID, "id", "", "Repo ID (kebab-slug); derived from name if omitted")
	cmd.Flags().StringVar(&flagName, "name", "", "Display name; defaults to directory basename")
	cmd.Flags().StringVar(&flagDefaultBranch, "default-branch", "", "Branch worktrees fork from; auto-resolved if omitted")

	return cmd
}
