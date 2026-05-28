package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cmux-board",
	Short: "Kanban board for cmux with Jira integration",
	Long: `cmux-board renders an issue-tracker kanban board in the terminal and lets
you activate any ticket to spawn a dedicated cmux workspace with a Claude Code
agent pane and a worktree shell pane.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
