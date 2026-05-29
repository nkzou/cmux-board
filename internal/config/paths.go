package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultConfigDir is the config directory relative to the user's home directory.
	DefaultConfigDir = ".config/cmux-board"
	// ConfigFileName is the name of the main config file.
	ConfigFileName = "config.json"
	// StateFileName is the name of the state file.
	StateFileName = "state.json"
	// CredentialsFileName is the name of the credentials file.
	CredentialsFileName = "credentials.json"
	// EnvCredentialsPath is the environment variable for overriding the credentials path.
	EnvCredentialsPath = "CMUX_BOARD_CREDENTIALS"
	// EnvConfigDir is the environment variable for overriding the config directory.
	// Used in tests and for non-default installations.
	EnvConfigDir = "CMUX_BOARD_CONFIG_DIR"
)

// ResolveCredentialsPath returns the path to credentials.json in this priority order:
// 1. flagVal (if non-empty, e.g. --credentials-from value)
// 2. CMUX_BOARD_CREDENTIALS environment variable
// 3. ~/.config/cmux-board/credentials.json default
func ResolveCredentialsPath(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if env := os.Getenv(EnvCredentialsPath); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(home, DefaultConfigDir, CredentialsFileName), nil
}

// ResolveConfigDir returns the config directory, honouring CMUX_BOARD_CONFIG_DIR if set.
func ResolveConfigDir() (string, error) {
	if env := os.Getenv(EnvConfigDir); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(home, DefaultConfigDir), nil
}

// DefaultConfigPath returns the path to config.json.
// Respects CMUX_BOARD_CONFIG_DIR if set.
func DefaultConfigPath() (string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFileName), nil
}

// DefaultStatePath returns the path to state.json.
// Respects CMUX_BOARD_CONFIG_DIR if set.
func DefaultStatePath() (string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, StateFileName), nil
}
