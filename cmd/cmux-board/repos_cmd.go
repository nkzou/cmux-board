package main

import "github.com/spf13/cobra"

var reposCmd = &cobra.Command{
	Use:   "repos",
	Short: "Manage registered repositories",
	Long:  `Manage the repository registry used for ticket activation and worktree creation.`,
}

func init() {
	reposCmd.AddCommand(newReposAddCmd())
	reposCmd.AddCommand(newReposRemoveCmd())
	reposCmd.AddCommand(newReposListCmd())
	rootCmd.AddCommand(reposCmd)
}
