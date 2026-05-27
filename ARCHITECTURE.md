# Architecture

## Overview

cmux-board is a Go 1.25 binary with a cobra CLI and BubbleTea TUI. It integrates
with Jira (via a tracker abstraction) and cmux (via JSON shellout) to spawn
claude-code agent sessions per ticket. Each ticket activation creates an isolated
git worktree, a background claude agent, and a cmux workspace with two panes.

## Module and binary

- Module: `github.com/nkzou/cmux-board`
- Binary: `cmux-board`
- Install: `go install github.com/nkzou/cmux-board/cmd/cmux-board@<commit>`

## Directory layout

```
cmd/cmux-board/        cobra CLI; init, dock, repos subcommands
internal/
  ui/                  BubbleTea Model, overlays, keymap, modes
  sync/                Poller, push handler, MergePulledTickets, bridge
  tracker/             IssueTracker port + Capabilities
  tracker/jira/        Jira Cloud v1 REST adapter
  cmuxcli/             cmux shellout wrappers (additive-only)
  claudecli/           claude shellout wrappers + RenderPrompt
  state/               Store, Mutate, Snapshot
  config/              Config/Credentials schemas, repos registry
  atomicfile/          Atomic write helper (broken out of config to avoid cycle)
  secretsink/          Writer-stage token redaction
  git/                 Worktree helpers (CreateWorktreeAt)
  runtime/             Pre-flight, shutdown, slog wiring, reconciliation
  initwizard/          First-run config wizard
```

## Key invariants

Enforced by `scripts/check_additive_only.sh`:

1. cmux is additive-only. Production code never calls `cmux close-workspace` or `cmux kill`.
2. `claude --bg` uses `exec.Cmd.Dir` for the working directory. `--cwd` is never used.
3. cmux focus uses `cmux focus-pane`. Bare `cmux focus` is not called.
4. `MergePulledTickets` is the only path that writes the `Tickets` map in state.
5. No direct `*State` field mutation outside store files -- all writes go through `store.Mutate`.
6. HTTP headers are never logged (token redaction boundary).
7. `git status` is never called in focus or activate paths (avoids dirty-worktree stalls).

## Persistence

See [docs/CONFIGURATION.md](docs/CONFIGURATION.md) for config schema and file locations.
See [docs/DATA_MODEL.md](docs/DATA_MODEL.md) for the state data model and Store API.

## Activation flow

See [docs/AGENT_INTEGRATION.md](docs/AGENT_INTEGRATION.md) for the full activation
sequence, cmux integration, and claude integration details.

## Out of scope (v1)

- Multi-tracker support (interface is defined; only the Jira adapter ships)
- Label editing, custom field editing, sprint management
- GitHub and Linear adapters
- Formal release pipeline (goreleaser, brew tap) -- file as a discovered bundle if needed
