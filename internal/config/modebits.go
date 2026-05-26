package config

import (
	"fmt"
	"log/slog"
	"os"
)

// ValidateCredentialsMode checks that credentials.json has mode <= 0600.
// If the file does not exist, no error is returned (first-run case).
// If unsafe is true, validation is skipped (for --unsafe-creds flag).
func ValidateCredentialsMode(path string, unsafe bool) error {
	if unsafe {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // file doesn't exist yet — OK
		}
		return fmt.Errorf("failed to stat credentials file: %w", err)
	}
	mode := info.Mode().Perm()
	if mode > 0600 {
		return fmt.Errorf("credentials file %s has unsafe mode %04o (want <= 0600); "+
			"run: chmod 600 %s", path, mode, path)
	}
	return nil
}

// EnsureConfigDirMode ensures the config directory has mode 0700.
// If the directory has looser permissions, chmod is attempted.
// On chmod failure, a warning is logged (not a fatal error).
func EnsureConfigDirMode(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("failed to stat config dir: %w", err)
	}
	mode := info.Mode().Perm()
	if mode > 0700 {
		if err := os.Chmod(dir, 0700); err != nil {
			slog.Warn("failed to tighten config directory permissions",
				"dir", dir, "current_mode", fmt.Sprintf("%04o", mode), "err", err)
		}
	}
	return nil
}
