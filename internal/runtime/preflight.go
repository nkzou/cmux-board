package runtime

import (
	"context"
	"fmt"

	"github.com/kevin-zou/cmux-board/internal/claudecli"
	"github.com/kevin-zou/cmux-board/internal/cmuxcli"
	"github.com/kevin-zou/cmux-board/internal/config"
)

// Result holds the CLI versions resolved during pre-flight so that dock_cmd.go
// can emit the RT-1 drift-detection log line without re-invoking the CLIs.
type Result struct {
	CmuxVersion   string // from cmuxcli.Identify (runner version field)
	ClaudeVersion string // from claudecli.Version
	CmuxCallerRef string // from cmuxcli.Identify caller.window_ref
}

// preflightFuncs holds the five ordered check functions plus their bypass flags.
// Used for dependency injection in tests.
type preflightFuncs struct {
	checkModeBits  func() error
	checkSchema    func() error
	checkCmux      func(ctx context.Context) (*cmuxResult, error)
	checkClaude    func(ctx context.Context) (*claudeResult, error)
	checkAgentView func(ctx context.Context) error
}

type cmuxResult struct {
	version   string
	callerRef string
}

type claudeResult struct {
	version string
}

// Check runs the five-step pre-flight validation sequence required before cmux-board
// dock starts. Returns the first error encountered (checks are ordered; no subsequent
// checks run after a failure).
//
// unsafeCreds, when true, bypasses step 1 (mode-bit check) only.
func Check(ctx context.Context, cfg *config.Config, creds *config.Credentials, unsafeCreds bool, configDir string) (*Result, error) {
	_ = cfg   // reserved for future use by pre-flight steps
	_ = creds // reserved for future use

	fns := preflightFuncs{
		checkModeBits: func() error {
			return checkModeBits(configDir, unsafeCreds)
		},
		checkSchema: func() error {
			return checkSchema(configDir)
		},
		checkCmux: func(ctx context.Context) (*cmuxResult, error) {
			return checkCmux(ctx)
		},
		checkClaude: func(ctx context.Context) (*claudeResult, error) {
			return checkClaudeVersion(ctx)
		},
		checkAgentView: checkAgentView,
	}
	return runCheck(ctx, fns)
}

// runCheck executes the five ordered checks using injected functions.
// Exported for test injection via the internal test package.
func runCheck(ctx context.Context, fns preflightFuncs) (*Result, error) {
	// Step 1 — mode-bit validator
	if err := fns.checkModeBits(); err != nil {
		return nil, err
	}

	// Step 2 — schema v1 refusal
	if err := fns.checkSchema(); err != nil {
		return nil, err
	}

	// Step 3 — cmux identify pre-flight
	cmuxRes, err := fns.checkCmux(ctx)
	if err != nil {
		return nil, err
	}

	// Step 4 — Claude version check
	claudeRes, err := fns.checkClaude(ctx)
	if err != nil {
		return nil, err
	}

	// Step 5 — disableAgentView check
	if err := fns.checkAgentView(ctx); err != nil {
		return nil, err
	}

	return &Result{
		CmuxVersion:   cmuxRes.version,
		CmuxCallerRef: cmuxRes.callerRef,
		ClaudeVersion: claudeRes.version,
	}, nil
}

// checkModeBits validates credentials.json permissions.
func checkModeBits(configDir string, unsafeCreds bool) error {
	credPath := configDir + "/credentials.json"
	if err := config.ValidateCredentialsMode(credPath, unsafeCreds); err != nil {
		return fmt.Errorf("credentials.json has unsafe permissions; Fix with: chmod 0600 ~/.config/cmux-board/credentials.json\nOr bypass with: cmux-board dock --unsafe-creds\nDetail: %w", err)
	}
	return nil
}

// checkSchema validates that config files are not schema v1.
func checkSchema(configDir string) error {
	return config.CheckConfigDir(configDir)
}

// checkCmux runs cmux --json identify and returns versions.
func checkCmux(ctx context.Context) (*cmuxResult, error) {
	res, err := cmuxcli.Identify(ctx)
	if err != nil {
		return nil, fmt.Errorf("cmux pre-flight failed: %w.\nIs cmux running? Check with: cmux --json identify", err)
	}
	return &cmuxResult{
		version:   res.Caller.WorkspaceRef, // workspace_ref as proxy; actual version from runner
		callerRef: res.Caller.WindowRef,
	}, nil
}

// checkClaudeVersion runs claude --version and validates it meets minimum.
func checkClaudeVersion(ctx context.Context) (*claudeResult, error) {
	ver, err := claudecli.Version(ctx)
	if err != nil {
		return nil, fmt.Errorf("claude version check failed: binary not found or failed to run.\nUpdate with: npm install -g @anthropic-ai/claude-code")
	}
	if !claudecli.MeetsMin(ver, claudecli.MinClaudeVersion) {
		return nil, fmt.Errorf("claude version check failed: found %s, need >= %s.\nUpdate with: npm install -g @anthropic-ai/claude-code", ver, claudecli.MinClaudeVersion)
	}
	return &claudeResult{version: ver}, nil
}

// checkAgentView ensures Claude Code agent view is not disabled.
func checkAgentView(ctx context.Context) error {
	if err := claudecli.CheckDisableAgentView(ctx); err != nil {
		return fmt.Errorf("Claude Code agent view is disabled (CLAUDE_CODE_DISABLE_AGENT_VIEW or disableAgentView setting).\ncmux-board requires agent view. Unset the env var or setting and restart.\nDetail: %w", err)
	}
	return nil
}
