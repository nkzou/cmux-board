package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kevin-zou/cmux-board/internal/config"
	"github.com/kevin-zou/cmux-board/internal/state"
)

func newReposRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Unregister a repository",
		Long: `Unregister a repository from cmux-board.

Refuses if any ticket has the id in assigned_repo_ids or any activation entry
references it. Clear assignments via the picker assignment editor (key 'a') first.

Note: removing a repo does not delete worktrees or git branches; those remain on disk
and must be cleaned up manually.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoID := args[0]

			cfgPath, err := config.DefaultConfigPath()
			if err != nil {
				return fmt.Errorf("failed to resolve config path: %w", err)
			}
			statePath, err := config.DefaultStatePath()
			if err != nil {
				return fmt.Errorf("failed to resolve state path: %w", err)
			}

			// Load config. Refuses at schema v1.
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Load state for reference scanning.
			// state.Open returns empty DefaultState on missing file.
			store, err := state.Open(statePath)
			if err != nil {
				return fmt.Errorf("failed to open state: %w", err)
			}
			st, _ := store.Snapshot()

			// RemoveRepo checks for references and mutates in-memory config (T-049).
			if err := config.RemoveRepo(&cfg, st, repoID); err != nil {
				return err // ErrRepoReferenced or "not found" — actionable message
			}

			// Atomic config write.
			if err := config.Save(cfgPath, cfg); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed: %s\n", repoID)
			return nil
		},
	}
}
