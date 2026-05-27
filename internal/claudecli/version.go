package claudecli

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// MinClaudeVersion is the minimum Claude Code version required.
// Bumped from spec's 2.1.141 per RESEARCH §11 — `agents --json` was broken before 2.1.150.
const MinClaudeVersion = "2.1.150"

var versionRE = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// Version runs `claude --version` and returns the parsed version string (e.g. "2.1.150").
func Version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "claude", "--version").Output()
	if err != nil {
		return "", fmt.Errorf("failed to run claude --version: %w", err)
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	m := versionRE.FindStringSubmatch(line)
	if m == nil {
		return "", fmt.Errorf("could not parse version from %q", line)
	}
	return m[1] + "." + m[2] + "." + m[3], nil
}

// MeetsMin returns true if actual >= min (semver comparison, major.minor.patch only).
func MeetsMin(actual, min string) bool {
	av := parseParts(actual)
	mv := parseParts(min)
	for i := 0; i < 3; i++ {
		if av[i] != mv[i] {
			return av[i] > mv[i]
		}
	}
	return true // equal
}

func parseParts(v string) [3]int {
	m := versionRE.FindStringSubmatch(v)
	if m == nil {
		return [3]int{}
	}
	var parts [3]int
	for i := 0; i < 3; i++ {
		parts[i], _ = strconv.Atoi(m[i+1])
	}
	return parts
}
