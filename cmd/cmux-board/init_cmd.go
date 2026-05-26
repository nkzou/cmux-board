package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize cmux-board configuration",
	Long:  "Interactive wizard to configure the Jira adapter, register repos, and write config files.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
