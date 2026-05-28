# cmux-board

**Module:** `github.com/nkzou/cmux-board`

TUI kanban board that syncs with Jira Cloud and spawns Claude Code agents per ticket.
Go 1.25+, BubbleTea, Lipgloss. Runs inside a cmux Dock sidebar.

## Structure

```
cmux-board/
cmd/cmux-board/      # CLI (Cobra): init, dock, repos
internal/
  ui/                # BubbleTea Model/View/Update (central orchestrator)
  sync/              # Poller, push handler, backoff, bridge, merge
  tracker/           # IssueTracker interface
  tracker/jira/      # Jira Cloud adapter
  state/             # State store (atomic JSON, Mutate/Snapshot)
  config/            # Config + credentials load/save, mode-bit validator
  claudecli/         # claude CLI wrapper (LaunchBackground, Agents, Respawn)
  cmuxcli/           # cmux CLI wrapper (NewWorkspace, FocusPane, ListWorkspaces)
  git/               # Worktree creation, branch sanitize, path uniquify
  runtime/           # Startup reconciliation, preflight checks, logger, shutdown
  initwizard/        # Interactive init wizard (prompts, finalize, dock snippet)
  secretsink/        # slog Writer-stage token redaction
docs/                # Design docs, smoke test, coverage table
```

## Where to Look

| Task | Location | Notes |
|------|----------|-------|
| Add keybinding | `internal/ui/keymap.go` | Dispatch by mode in `handleNormalMode()` |
| New UI mode | `internal/ui/model.go` | Add Mode const, create handler |
| Change poll behavior | `internal/sync/poller.go` | Backoff in `internal/sync/backoff.go` |
| Card-move push | `internal/sync/push.go` | OCC emulator; DryRun gate |
| Activation flow | `internal/ui/activate.go` | ActivationHooks fault-injection seam |
| Jira client | `internal/tracker/jira/client.go` | Auth header injected via `Do()` |
| State mutations | `internal/state/store.go` | ALL mutations via `store.Mutate()` |
| Startup reconcile | `internal/runtime/reconcile.go` | Discovers resources by `act_id_short` |
| Config paths | `internal/config/paths.go` | Flag > env > default cascade |

## Code Map

| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `Model` | struct | `internal/ui/model.go` | Main app state, BubbleTea model |
| `Update()` | method | `internal/ui/model.go` | Event handler (NEVER BLOCK) |
| `View()` | method | `internal/ui/view.go` | All rendering |
| `Store` | struct | `internal/state/store.go` | Atomic JSON store (Mutate/Snapshot) |
| `ActivationHooks` | struct | `internal/ui/activate.go` | Fault-injection seam |
| `Bridge` | struct | `internal/sync/bridge.go` | BubbleTea-poller channel bridge |
| `IssueTracker` | interface | `internal/tracker/tracker.go` | Adapter contract |
| `Client` | struct | `internal/tracker/jira/client.go` | Jira HTTP client with auth |
| `ReconcileIncompleteActivations` | func | `internal/runtime/reconcile.go` | Startup resource harvest |

## Conventions

- Imports: stdlib, blank, external, blank, internal.
- Errors: return last; wrap with `fmt.Errorf("context: %w", err)`.
- ALL `state.json` mutations go through `store.Mutate(func(s *state.State) error {...})`.
- Token redaction: register raw token once via `secretsink.Register(token)`, then wrap every
  `io.Writer` that receives slog output with `secretsink.Writer(w)`.
- cmux commands: ONLY additive operations. Never `close-workspace`, `close-window`, `kill`.
- Claude invocation: use `cmd.Dir = worktreePath`, NOT `--cwd` flag.
- cmux focus: use `cmux focus-pane --workspace W --pane P`, NOT bare `cmux focus`.

## Anti-Patterns

- NEVER block in `Update()`. All I/O via `tea.Cmd`.
- NEVER log HTTP headers (Authorization, Cookie). See F20.d.
- NEVER call `cmux close-*`, `cmux kill`, or any destructive workspace command.
- NEVER pass `--cwd` to `claude`. Use `cmd.Dir`.
- NEVER adopt a cmux workspace by `current_directory` matching alone. Only by cached ID.
- NEVER mutate `state.State` outside of `store.Mutate()`.

## Commands

```bash
make build             # Build binary (./cmux-board)
make test              # go test -race ./... + verify-additive
make verify-additive   # scripts/check_additive_only.sh (7 grep arms)
go vet ./...           # Vet
staticcheck ./...      # Static analysis
go test -race ./...    # Race detector (mandatory)
```

## Testing

Config directory override for test isolation:

```bash
CMUX_BOARD_CONFIG_DIR=/tmp/test cmux-board dock
```

All tests in `internal/state/` that import `internal/sync` must use `package state_test`
(external test package) to avoid circular imports.

Race detector is mandatory: `go test -race ./...` must pass.

See [docs/acceptance-coverage.md](docs/acceptance-coverage.md) for criterion-to-test mapping.

## Data Locations

| Data | Path | Mode |
|------|------|------|
| Config | `~/.config/cmux-board/config.json` | 0644 |
| Credentials | `~/.config/cmux-board/credentials.json` | 0600 |
| State | `~/.config/cmux-board/state.json` | 0644 |
| Config dir | `~/.config/cmux-board/` | 0700 |
