package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/runtime"
	"github.com/nkzou/cmux-board/internal/state"
	isync "github.com/nkzou/cmux-board/internal/sync"
	"github.com/nkzou/cmux-board/internal/tracker"
	"github.com/nkzou/cmux-board/internal/tracker/jira"
	"github.com/nkzou/cmux-board/internal/ui"
)

var dockCmd = &cobra.Command{
	Use:   "dock",
	Short: "Start the cmux-board kanban Dock",
	Long:  "Launch the cmux-board kanban board in a cmux Dock sidebar.",
	RunE:  runDock,
}

func init() {
	dockCmd.Flags().Bool("unsafe-creds", false,
		"bypass credentials.json mode-bit check (for development only)")
	dockCmd.Flags().String("log-level", "info",
		"log level: debug, info, warn, error")
	rootCmd.AddCommand(dockCmd)
}

// dockDeps holds injectable dependencies for testing. Production code uses prodDockDeps.
type dockDeps struct {
	// loadConfig loads config.json from the given path.
	loadConfig func(path string) (config.Config, error)
	// loadCredentials loads credentials.json from the given path.
	loadCredentials func(path string) (config.Credentials, error)
	// check runs the pre-flight chain.
	check func(ctx context.Context, cfg *config.Config, creds *config.Credentials, unsafeCreds bool, configDir string) (*runtime.Result, error)
	// newLogger installs the global slog logger.
	newLogger func(level slog.Level, w io.Writer) *slog.Logger
	// openState opens state.json.
	openState func(path string) (*state.Store, error)
	// reconcile runs startup reconciliation.
	reconcile func(ctx context.Context, store *state.Store, cfg *config.Config) error
	// checkACLI verifies acli is installed and authenticated before starting.
	// Returns an error with an actionable message if acli is absent or unauthenticated.
	checkACLI func(ctx context.Context) error
	// newTracker constructs the issue tracker from config and credentials.
	newTracker func(cfg config.Config, creds config.Credentials) tracker.IssueTracker
	// runProgram runs the BubbleTea program and returns when the user quits.
	// resultCh, when non-nil, is drained and forwarded to the program via Send.
	// Production passes bridge.ResultCh; tests pass nil (no-op).
	runProgram func(ctx context.Context, cfg *config.Config, store *state.Store, resultCh <-chan tea.Msg) error
	// logWriter is the underlying writer for the logger (nil = os.Stderr).
	logWriter io.Writer
}

// prodDockDeps returns the production dependency set.
func prodDockDeps() dockDeps {
	return dockDeps{
		loadConfig:      config.Load,
		loadCredentials: config.LoadCredentials,
		check:           runtime.Check,
		newLogger: func(level slog.Level, w io.Writer) *slog.Logger {
			if w == nil {
				return runtime.NewLogger(level)
			}
			return runtime.NewLoggerWithWriter(level, w)
		},
		openState: state.Open,
		reconcile: func(ctx context.Context, store *state.Store, cfg *config.Config) error {
			return runtime.ReconcileIncompleteActivations(ctx, store, cfg)
		},
		checkACLI: checkACLIPreflight,
		newTracker: func(cfg config.Config, creds config.Credentials) tracker.IssueTracker {
			jiraCreds, ok := creds.Adapters["jira"]
			if !ok {
				return nil
			}
			// Parse manually-configured columns from adapter_config.columns.
			cfgCols := config.ParseAdapterColumns(cfg.AdapterConfig)
			trackerCols := make([]tracker.Column, len(cfgCols))
			for i, c := range cfgCols {
				trackerCols[i] = tracker.Column{
					ID:        c.Name, // use name as ID; Resolve compares case-insensitively
					Name:      c.Name,
					StatusIDs: c.Statuses,
				}
			}
			return jira.NewJiraAdapter(jira.Config{
				Site:    jiraCreds.SiteURL,
				Columns: trackerCols,
			})
		},
		runProgram: func(ctx context.Context, cfg *config.Config, store *state.Store, resultCh <-chan tea.Msg) error {
			model := ui.NewModelWithContext(ctx, cfg, store)
			prog := tea.NewProgram(model, tea.WithAltScreen(), tea.WithOutput(os.Stderr))
			// Drain bridge.ResultCh and forward each message to the program.
			if resultCh != nil {
				go func() {
					for {
						select {
						case <-ctx.Done():
							return
						case msg, ok := <-resultCh:
							if !ok {
								return
							}
							prog.Send(msg)
						}
					}
				}()
			}
			_, err := prog.Run()
			return err
		},
		logWriter: nil,
	}
}

func runDock(cmd *cobra.Command, args []string) error {
	return runDockWithDeps(cmd, args, prodDockDeps())
}

// runDockWithDeps implements the full dock startup sequence with injected dependencies.
// This function is the unit-testable core; runDock wraps it with production deps.
func runDockWithDeps(cmd *cobra.Command, _ []string, deps dockDeps) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	unsafeCreds, _ := cmd.Flags().GetBool("unsafe-creds")
	logLevelStr, _ := cmd.Flags().GetString("log-level")

	// Resolve config directory and paths.
	configDir, err := resolveRootConfigDir()
	if err != nil {
		return fmt.Errorf("failed to resolve config directory: %w", err)
	}
	configPath := configDir + "/" + config.ConfigFileName
	credsPath := configDir + "/" + config.CredentialsFileName
	statePath := configDir + "/" + config.StateFileName

	// Step 1: Load config.json (schema v1 check via PersistentPreRunE already ran).
	cfg, err := deps.loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Load credentials.
	creds, err := deps.loadCredentials(credsPath)
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}

	// Step 2: (acli adapter) No API token to register — acli owns credentials.
	// secretsink stays active for any other adapters or future secrets.
	_ = creds

	// Step 3: Install global slog logger with secretsink redaction active.
	deps.newLogger(resolveLogLevel(logLevelStr), deps.logWriter)

	// Step 4: pre-flight chain — five ordered checks.
	preflightResult, err := deps.check(ctx, &cfg, &creds, unsafeCreds, configDir)
	if err != nil {
		return err
	}

	// Step 5: Load state.json.
	store, err := deps.openState(statePath)
	if err != nil {
		return fmt.Errorf("failed to open state: %w", err)
	}

	// Step 6: Drift-detection log line (RT-1).
	// Versions come from preflightResult — do NOT re-invoke the CLIs.
	slog.Debug("startup",
		"cmux_version", preflightResult.CmuxVersion,
		"claude_version", preflightResult.ClaudeVersion,
		"cmux_caller_ref", preflightResult.CmuxCallerRef,
		"started_at", time.Now().UTC().Format(time.RFC3339),
	)

	// Step 7: Startup reconciliation — harvest orphans from prior crash.
	if err := deps.reconcile(ctx, store, &cfg); err != nil {
		// Non-fatal: log and continue.
		slog.Warn("startup reconciliation encountered errors", "err", err)
	}

	// Step 7.5: acli pre-flight — verify acli is installed and authenticated.
	if deps.checkACLI != nil {
		if err := deps.checkACLI(ctx); err != nil {
			return err
		}
	}

	// Construct the tracker adapter.
	tr := deps.newTracker(cfg, creds)

	// Step 8: Create bridge and start sync.Poller + push worker.
	// bridge.ResultCh is drained inside runProgram via prog.Send.
	bridge := isync.NewBridge()
	poller := isync.NewPoller(ctx, &cfg, tr, store, bridge.EmitFn())
	go bridge.RunPushWorker(ctx, store, tr)

	// Shutdown coordinator.
	coord := runtime.NewCoordinator(cancel, poller, store, slog.Default())

	// Step 9: Run BubbleTea program (blocks until user presses q).
	// bridge.ResultCh draining is handled inside runProgram via prog.Send.
	if err := deps.runProgram(ctx, &cfg, store, bridge.ResultCh); err != nil {
		slog.Error("bubbletea program exited with error", "err", err)
	}

	// Step 10: BubbleTea exited → cancel context → drain → flush.
	// Cancel first so the Coordinator.Run sees ctx.Done() and proceeds with drain.
	cancel()
	coord.Run(ctx)

	return nil
}

// checkACLIPreflight verifies that acli is installed and authenticated.
// Runs 'acli jira auth status' and returns an actionable error if the check fails.
// This is called before the adapter is constructed so startup fails fast with a clear message.
func checkACLIPreflight(ctx context.Context) error {
	adapter := jira.NewJiraAdapter(jira.Credentials{})
	_, err := adapter.WhoAmI(ctx)
	if err != nil {
		return fmt.Errorf("acli pre-flight failed: %w\nRun 'acli jira auth login --web' and retry", err)
	}
	return nil
}

// resolveLogLevel converts the --log-level flag string to a slog.Level.
// Unknown values default to LevelInfo.
func resolveLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
