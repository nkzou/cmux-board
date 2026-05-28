# cmux-board

Terminal-based kanban board with Jira sync and Claude Code agent spawning. Runs inside a cmux workspace.

## Stack

Go 1.25+, BubbleTea (TUI), Lipgloss

## Development

```bash
make build            # Build binary
make test             # go test -race ./... + verify-additive
go vet ./...          # Vet
staticcheck ./...     # Static analysis
```

## Where to Look

| Task | Location |
|------|----------|
| Add CLI command | `cmd/cmux-board/` |
| Modify UI/keybindings | `internal/ui/` |
| Change sync/poll behavior | `internal/sync/` |
| Jira adapter | `internal/tracker/jira/` |
| State persistence | `internal/state/` |
| Configuration | `internal/config/` |
| Git operations | `internal/git/` |
| Startup/reconcile | `internal/runtime/` |
| Init wizard | `internal/initwizard/` |

## Architecture

```
cmd/cmux-board/      CLI entry (cobra): init, dock, repos
internal/
  ui/                BubbleTea Model - central orchestrator
  sync/              Poller, push, backoff, bridge, merge
  tracker/           IssueTracker interface
  tracker/jira/      Jira Cloud adapter
  state/             Atomic JSON store (Mutate/Snapshot)
  config/            Config, credentials, mode-bit validator
  claudecli/         claude CLI wrapper
  cmuxcli/           cmux CLI wrapper
  git/               Worktree, branch naming, path uniquify
  runtime/           Startup reconcile, preflight, shutdown, logger
  initwizard/        Interactive init wizard
  secretsink/        slog Writer-stage token redaction
```

## Key Flows

**Ticket activation:**
`ui.handleEnter()` -> `activate()` -> `git.CreateWorktreeAt()` -> `claudecli.LaunchBackground()` -> `cmuxcli.NewWorkspaceWithLayout()` -> `cmuxcli.FocusPane()`

**Background poll:**
`sync.NewPoller()` -> `bridge.EmitFn()` -> `bridge.RunPushWorker()` -> `tea.Program.Send()`

**State mutation:**
Any goroutine -> `store.Mutate(func(s *state.State) error {...})` -> atomic JSON write

## Guidance

Context-specific guidance lives in nested CLAUDE.md files:
- `internal/CLAUDE.md` - Go patterns, imports, testing
- `internal/ui/CLAUDE.md` - BubbleTea patterns
