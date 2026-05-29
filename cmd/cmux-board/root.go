package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
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
		dir, err := config.ResolveConfigDir()
		if err != nil {
			return err
		}
		return config.CheckConfigDir(dir)
	}
}

// loadConfigForCmd resolves the default config path and loads it. Shared by the
// repos subcommands. Load refuses at schema v1.
func loadConfigForCmd() (config.Config, string, error) {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return config.Config{}, "", fmt.Errorf("failed to resolve config path: %w", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return config.Config{}, "", fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, cfgPath, nil
}

// openStateForCmd opens the default state store. state.Open returns an empty
// DefaultState on a missing file.
func openStateForCmd() (*state.Store, error) {
	statePath, err := config.DefaultStatePath()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve state path: %w", err)
	}
	store, err := state.Open(statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open state: %w", err)
	}
	return store, nil
}
