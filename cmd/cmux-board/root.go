package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nkzou/cmux-board/internal/config"
)

func init() {
	// PersistentPreRunE runs before every subcommand (including nested ones like repos add).
	// It enforces the schema v1 refusal check so the binary cannot operate on stale v1 config.
	// The "init" subcommand is exempted because it writes fresh v2 config.
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// "init" writes new config — skip the check to allow a fresh install.
		if cmd.Name() == "init" {
			return nil
		}
		dir, err := resolveRootConfigDir()
		if err != nil {
			return err
		}
		return config.CheckConfigDir(dir)
	}
}

// resolveRootConfigDir returns the config dir, honouring CMUX_BOARD_CONFIG_DIR env var.
func resolveRootConfigDir() (string, error) {
	if env := os.Getenv(config.EnvConfigDir); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, config.DefaultConfigDir), nil
}
