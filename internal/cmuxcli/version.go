package cmuxcli

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// knownGoodMinVersion is the minimum cmux version cmux-board has been tested against.
// Startup emits a slog.Warn when the installed version is older (RT-1 drift detection).
const knownGoodMinVersion = "0.64.7"

// Version calls `cmux --version` and returns the trimmed version string.
// The exact argv is: cmux --version
func Version(ctx context.Context) (string, error) {
	stdout, _, err := runCmux(ctx, "--version")
	if err != nil {
		return "", fmt.Errorf("failed to get cmux version: %w", err)
	}
	return strings.TrimSpace(string(stdout)), nil
}

// CheckVersion calls Version and logs a warning if the version predates knownGoodMinVersion.
// It never blocks startup — the warning is informational only (RT-1).
func CheckVersion(ctx context.Context) {
	v, err := Version(ctx)
	if err != nil {
		slog.WarnContext(ctx, "cmux version check failed", "error", err)
		return
	}
	slog.InfoContext(ctx, "cmux version", "version", v)
	if !meetsMinVersion(v, knownGoodMinVersion) {
		slog.WarnContext(ctx, "cmux version predates known-good range; CLI surface may have changed",
			"installed", v, "known_good_min", knownGoodMinVersion)
	}
}

// meetsMinVersion returns true when installed >= min (simple dot-split comparison).
// Both strings may have a leading "cmux " prefix or trailing build metadata; only the
// first dot-separated number sequence is compared.
func meetsMinVersion(installed, min string) bool {
	iParts := splitVersion(installed)
	mParts := splitVersion(min)
	for i := range mParts {
		if i >= len(iParts) {
			return false
		}
		if iParts[i] > mParts[i] {
			return true
		}
		if iParts[i] < mParts[i] {
			return false
		}
	}
	return true
}

// splitVersion parses the leading "X.Y.Z" component of a version string into ints.
// Handles "cmux 0.64.7 (87) [hash]" and "0.64.7" alike.
func splitVersion(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "cmux ")
	// Take everything before the first space (drops "(87) [hash]" suffix).
	head, _, _ := strings.Cut(v, " ")
	var out []int
	for _, seg := range strings.Split(head, ".") {
		n, err := strconv.Atoi(strings.TrimSpace(seg))
		if err != nil {
			// Non-numeric segment — treat as 0 to make comparison deterministic.
			n = 0
		}
		out = append(out, n)
	}
	return out
}
