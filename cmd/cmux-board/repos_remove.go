package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkzou/cmux-board/internal/config"
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

			// Load config (refuses at schema v1) and state for reference scanning.
			cfg, cfgPath, err := loadConfigForCmd()
			if err != nil {
				return err
			}
			store, err := openStateForCmd()
			if err != nil {
				return err
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
