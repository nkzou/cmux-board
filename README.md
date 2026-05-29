# cmux-board

A TUI kanban board that runs inside a [cmux](https://cmux.dev) workspace. Syncs with Jira Cloud and spawns Claude Code agents per ticket, each in its own git worktree and cmux workspace.

---

## What it does

cmux-board keeps a local mirror of your Jira board. Press `Enter` on a ticket to:

1. Create a git worktree for that ticket.
2. Launch a Claude Code background session (`claude --bg`) in the worktree.
3. Open a cmux workspace with the agent view on the left and a shell on the right.
4. Focus the workspace automatically.

A background poller keeps the board in sync. Pushing a card to a different column transitions it in Jira immediately. Conflicts snap the card back with a toast notification.

You can also create local-only tickets (no Jira sync) and import Jira tickets by key directly from the board.

---

## Prerequisites

- [cmux](https://cmux.dev) installed and running
- [Claude Code](https://claude.ai/code) CLI v2.1.150 or later
- [acli](https://acli.dev) installed and authenticated to Jira Cloud
- Go 1.25+ (build from source only)

---

## Install

```bash
git clone https://github.com/nkzou/cmux-board
cd cmux-board
make build
# binary: ./cmux-board
```

Move the binary somewhere on your `PATH`:

```bash
mv cmux-board /usr/local/bin/
```

---

## Quick start

See [docs/getting-started.md](docs/getting-started.md) for a full walkthrough.

```bash
# First-time setup
cmux-board init

# Start the board in a cmux workspace
cmux-board dock
```

`cmux-board init` runs an interactive wizard that:
- Authenticates to Jira Cloud via acli.
- Picks a board and configures columns.
- Registers one or more git repos.
- Writes `~/.config/cmux-board/config.json` and `credentials.json` (mode 0600).

---

## CLI reference

### `cmux-board init`

Interactive wizard for first-time setup or reconfiguration.

| Flag | Description |
|------|-------------|
| `--force` | Overwrite config and credentials; preserve state.json |
| `--reset` | Remove all config files and re-run wizard from scratch |
| `--adapter` | Adapter name (`jira`); skips adapter prompt |
| `--site` | Jira site URL (e.g. `yourorg.atlassian.net`) |
| `--board-id` | Board ID; skips interactive board selection |
| `--repo` | Repo paths to register (repeat or comma-separate) |
| `--non-interactive` | Run without prompts; all values from flags |

### `cmux-board dock`

Start the TUI board. Blocks until `q`.

| Flag | Description |
|------|-------------|
| `--log-level` | Log level: `debug`, `info`, `warn`, `error` (default: `info`) |
| `--debug` | Show mouse-event counters and drag diagnostics in the status bar |
| `--unsafe-creds` | Bypass credentials.json mode-bit check (development only) |

### `cmux-board repos add <path>`

Register a git repository.

| Flag | Description |
|------|-------------|
| `--id` | Repo ID (kebab-slug); auto-derived from directory name if omitted |
| `--name` | Display name; defaults to directory basename |
| `--default-branch` | Base branch for new worktrees; auto-resolved if omitted |

### `cmux-board repos list`

List registered repositories.

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON array |

### `cmux-board repos remove <id>`

Unregister a repository. Refuses if any ticket or activation references it.

---

## Key bindings

### Normal mode

| Key | Action |
|-----|--------|
| `h` / `l` | Move cursor left / right across columns |
| `j` / `k` | Move cursor down / up within a column |
| `Tab` / `Shift+Tab` | Cycle through tickets by z-order |
| `Enter` | Activate ticket or focus existing activation |
| `N` | New activation with a named approach |
| `m` | Open activation picker for current ticket |
| `a` | Open repo assignment editor |
| `i` | Import a Jira ticket by key |
| `c` | Create a local ticket |
| `x` | Remove ticket from the board |
| `s` | Cycle status (local tickets) |
| `/` | Filter tickets by summary substring |
| `?` | Show help overlay |
| `q` | Quit |

### Picker mode (activation selector)

| Key | Action |
|-----|--------|
| `Enter` | Focus selected activation |
| `n` | Create new approach from picker |
| `r` | Respawn selected activation |
| `d` | Delete selected activation |
| `j` / `k` or arrows | Navigate entries |
| `Esc` | Close picker |

### Assignment editor

| Key | Action |
|-----|--------|
| `Space` | Toggle repo selection |
| `j` / `k` or arrows | Navigate repos |
| `Enter` | Commit changes |
| `Esc` | Cancel |

### Approach name input

| Key | Action |
|-----|--------|
| `Enter` | Create activation with entered name |
| `Esc` | Cancel |

---

## Configuration

All config lives in `~/.config/cmux-board/`. Override the directory with `CMUX_BOARD_CONFIG_DIR`.

| File | Mode | Contents |
|------|------|----------|
| `config.json` | 0644 | Board settings, repos, poll interval |
| `credentials.json` | 0600 | Jira API token and email |
| `state.json` | 0644 | Ticket mirror and activation journal |

### config.json

```json
{
  "schema_version": 2,
  "adapter": "jira",
  "adapter_config": {
    "site_url": "https://yourorg.atlassian.net",
    "board_id": "42",
    "columns": [
      { "name": "To Do",       "statuses": ["To Do"] },
      { "name": "In Progress", "statuses": ["In Progress"] },
      { "name": "Done",        "statuses": ["Done"] }
    ]
  },
  "repos": {
    "my-service": {
      "id": "my-service",
      "name": "my-service",
      "path": "/absolute/path/to/repo",
      "default_branch": "main"
    }
  },
  "poll_interval_seconds": 60,
  "worktree_base_dir": "~/.config/cmux-board/worktrees",
  "claude": {
    "model": "claude-opus-4-5",
    "permission_mode": "bypassPermissions",
    "starter_prompt": ""
  },
  "dry_run": false
}
```

Notable fields:

- `poll_interval_seconds`: minimum 10 seconds; values below are clamped with a warning.
- `claude.model`: passed as `--model` to `claude --bg`. Omit to use Claude's default.
- `claude.permission_mode`: passed as `--permission-mode`. Common values: `bypassPermissions`, `default`.
- `claude.starter_prompt`: Go template injected as the opening prompt. Leave empty for the built-in default.
- `dry_run`: when `true`, Jira transitions are skipped. Card moves snap back with a toast.

### Starter prompt template variables

When customizing `claude.starter_prompt`, these variables are available:

| Variable | Value |
|----------|-------|
| `{{.Ticket.Key}}` | Ticket key (e.g. `PROJ-42`) |
| `{{.Ticket.Summary}}` | Ticket title |
| `{{.Ticket.Status}}` | Current Jira status |
| `{{.Ticket.Priority}}` | Priority field |
| `{{.Ticket.IssueType}}` | Issue type (Bug, Story, etc.) |
| `{{.Ticket.AssigneeEmail}}` | Assignee email |
| `{{.Ticket.Labels}}` | Labels slice |
| `{{.Ticket.URL}}` | Jira URL |
| `{{.Repo.Name}}` | Repo display name |
| `{{.Repo.Path}}` | Absolute repo path |
| `{{.WorktreePath}}` | Absolute worktree path |
| `{{.ApproachName}}` | Approach name entered by user |

---

## Repo management

```bash
cmux-board repos add /path/to/repo   # Register a repo
cmux-board repos list                 # List registered repos
cmux-board repos list --json          # JSON output
cmux-board repos remove <id>          # Remove (refused if referenced)
```

A ticket can be assigned to multiple repos. On activation, if a ticket has more than one assigned repo, a picker opens to select which repo to use.

---

## Activation flow

Pressing `Enter` on a ticket:

1. Generates a ULID activation ID.
2. Creates a git worktree at `<worktree_base_dir>/<repo>-<ticket>-<approach>-<id>/`.
3. Launches `claude --bg --name <id>-<ticket>-<approach>` with the starter prompt.
4. Creates a cmux workspace: agent pane on the left, shell on the right.
5. Focuses the agent pane.

Each step is journaled before executing. If the process crashes mid-activation, the next `cmux-board dock` startup reconciles the incomplete entry against live cmux workspaces and Claude sessions.

---

## Development

```bash
make build            # Build binary
make test             # go test -race ./... + verify-additive
go vet ./...          # Vet
staticcheck ./...     # Static analysis
```

See [docs/smoke-test.md](docs/smoke-test.md) for the manual end-to-end checklist.

---

## Credits

Based on [openkanban](https://github.com/TechDufus/openkanban) by TechDufus. The BubbleTea board view in `internal/ui/` and the git-worktree helpers in `internal/git/` were ported from it.

---

## License

[AGPL-3.0](LICENSE)
