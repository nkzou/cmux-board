package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var dockCmd = &cobra.Command{
	Use:   "dock",
	Short: "Start the cmux-board kanban Dock",
	Long:  "Launch the cmux-board kanban board in a cmux Dock sidebar.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dockCmd)
}
