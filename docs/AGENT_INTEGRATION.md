# Agent Integration

cmux-board spawns one cmux workspace per ticket activation. Each workspace has two
panes: a claude agent pane (live-attached via `claude attach <act_id_short>`) and a
worktree shell pane.

## cmux integration

cmux is additive-only. The binary never calls `close-workspace`, `kill`, or any
`workspace-action --action close-*` variant. See `internal/cmuxcli/` for the
`// Never call:` comment and the typed wrappers.

Subcommands used:

- `cmux --json identify` -- verify cmux is available and reachable
- `cmux new-workspace --name <ws_name>` -- create the two-pane workspace
- `cmux focus-pane --workspace <ws_ref> --pane <pane_ref>` -- focus a specific pane
- `cmux docs dock` -- open the dock sidebar

## claude integration

Minimum version: 2.1.150 (required for `--json` stability).

- Background spawn: `claude --bg --name <act_id_short>-<ticket>-<approach>`
  with `exec.Cmd.Dir = worktreePath`. The `--cwd` flag is never used (rejected on 2.1.150).
- Live attach: `claude attach <act_id_short>` in the agent pane.
- Version check: `claude --version` is called at startup via `internal/claudecli/`.
- `CLAUDE_CODE_DISABLE_AGENT_VIEW` is checked at startup; if set, the binary refuses to start.

## Activation flow

```
activate(ticket)
  -> generate activation_id (ULID)
  -> journal activation_id BEFORE any side effect (state.Store.Mutate)
  -> create worktree at <base>/<repo_id>-<ticket>-<approach>-<act_id_short>/
  -> claude --bg --name <act_id_short>-... (exec.Cmd.Dir = worktree)
  -> cmux new-workspace --name <act_id_short>-...
  -> journal completion (state.Store.Mutate)
```

Each side effect is journaled before the next one begins. An interrupted activation
leaves a `complete: false` entry in state that reconciliation can recover.

## Reconciliation

On startup, `internal/runtime/reconcile.go` scans live cmux workspaces and claude
sessions for the `act_id_short` substring. Journaled-but-incomplete activations are
matched to their external resources without needing to store separate external IDs at
journal time. This resolves the gap where an activation crashes between side effects.

## Cross-references

- `internal/claudecli/` -- claude shellout wrappers
- `internal/cmuxcli/` -- cmux shellout wrappers
- `internal/ui/activate.go` -- activation orchestrator
- `internal/runtime/reconcile.go` -- startup reconciliation
