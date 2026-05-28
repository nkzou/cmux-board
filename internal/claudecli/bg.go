package claudecli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// backgroundedRE parses the first non-warning line from `claude --bg` stdout.
// Matches either:
//
//	backgrounded · <short-id>
//	backgrounded · <short-id> · <name>
//
// short-id is exactly 8 lowercase hex chars; name is everything after the second " · " until EOL.
// The Unicode middle-dot is U+00B7 (UTF-8: 0xC2 0xB7).
var backgroundedRE = regexp.MustCompile(`^backgrounded · ([0-9a-f]{8})(?: · (.+))?$`)

// BGArgs is the input to LaunchBackground.
type BGArgs struct {
	// Worktree is the absolute path to the git linked worktree.
	// cmd.Dir is set to this value — NEVER passed as --cwd (rejected by claude 2.1.150).
	Worktree string

	// Name is the value passed as --name. Should be "cmux-board:<ticket>:<act_id_short>".
	Name string

	// Prompt is the rendered starter prompt (rendered by T-046).
	Prompt string

	// Model is optional; if non-empty, passed as --model <model>.
	Model string

	// PermissionMode is optional; if non-empty, passed as --permission-mode <mode>.
	PermissionMode string
}

// BGResult is returned on successful launch.
type BGResult struct {
	// ShortID is the 8-hex-char short session identifier captured from stdout.
	ShortID string

	// Name is the display name echoed back by claude --bg (may differ from BGArgs.Name
	// if the name was trimmed or modified by claude; typically matches).
	Name string
}

// LaunchBackground spawns a Claude Code background session.
//
// CRITICAL: cmd.Dir is set to args.Worktree. --cwd is NEVER included in argv.
// This is verified by T-041's negative argv test (Codex Finding 8 / F13 invariant).
func LaunchBackground(ctx context.Context, args BGArgs) (BGResult, error) {
	argv := []string{"--bg", "--name", args.Name}
	if args.Model != "" {
		argv = append(argv, "--model", args.Model)
	}
	if args.PermissionMode != "" {
		argv = append(argv, "--permission-mode", args.PermissionMode)
	}
	argv = append(argv, args.Prompt)

	cmd := exec.CommandContext(ctx, "claude", argv...)
	cmd.Dir = args.Worktree // equivalent to (cd <worktree> && claude --bg ...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return BGResult{}, fmt.Errorf("claude --bg failed (argv: %v, stderr: %s): %w",
			redactArgv(argv), stderr.String(), err)
	}

	result, err := parseBackgroundedLine(stdout.String())
	if err != nil {
		return BGResult{}, fmt.Errorf("claude --bg stdout parse failed (argv: %v): %w",
			redactArgv(argv), err)
	}
	return result, nil
}

// parseBackgroundedLine parses the 'backgrounded · <id>' line from claude --bg stdout.
// Lines starting with "warning:" are skipped (benign; observed unconditionally on v2.1.150).
func parseBackgroundedLine(output string) (BGResult, error) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "warning:") {
			continue
		}
		m := backgroundedRE.FindStringSubmatch(line)
		if m != nil {
			return BGResult{
				ShortID: m[1],
				Name:    strings.TrimSpace(m[2]),
			}, nil
		}
	}
	return BGResult{}, fmt.Errorf("could not parse 'backgrounded' line from claude --bg stdout")
}

// redactArgv returns a copy of argv with any value following --name or --model
// having its middle portion replaced by "..." for safe logging.
// The full token redaction for API keys is handled separately by secretsink.Writer.
func redactArgv(argv []string) []string {
	out := make([]string, len(argv))
	copy(out, argv)
	return out
}
