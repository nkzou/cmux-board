# cmux-board

A TUI kanban board that runs inside a [cmux](https://cmux.dev) Dock sidebar. Syncs with Jira Cloud and spawns Claude Code agents per ticket, each in its own git worktree and cmux workspace.

---

## What it does

cmux-board keeps a local mirror of your Jira board. Press `Enter` on a ticket to:

1. Create a git worktree for that ticket.
2. Launch a Claude Code background session (`claude --bg`) in the worktree.
3. Open a cmux workspace with the agent view on the left and a shell on the right.
4. Focus the workspace automatically.

A background poller keeps the board in sync. Pushing a card to a different column transitions it in Jira immediately. Conflicts snap the card back with a toast.

---

## Install

Build from source:

```bash
git clone https://github.com/nkzou/cmux-board
cd cmux-board
make build
# binary: ./cmux-board
```

---

## Quick start

```bash
# First-time setup
cmux-board init

# Start the board inside a cmux Dock pane
cmux-board dock
```

`cmux-board init` runs an interactive wizard that:
- Authenticates to Jira Cloud.
- Picks a board.
- Registers one or more git repos.
- Writes `~/.config/cmux-board/config.json` and `credentials.json` (mode 0600).
- Prints the Dock snippet to add to `~/.config/cmux/dock.json`.

---

## Key bindings

| Key | Action |
|-----|--------|
| `j / k` | Move cursor up / down |
| `h / l` | Move cursor left / right across columns |
| `Enter` | Activate ticket (create worktree + agent) or focus existing |
| `N` | New activation with a named approach |
| `a` | Open repo assignment editor |
| `q` | Quit |
| `?` | Help |

---

## Repo management

```bash
cmux-board repos add /path/to/repo   # Register a repo
cmux-board repos list                 # List registered repos
cmux-board repos remove <id>          # Remove (refused if referenced)
```

---

## Configuration

All config lives in `~/.config/cmux-board/`:

| File | Mode | Contents |
|------|------|----------|
| `config.json` | 0644 | Board settings, repos, poll interval |
| `credentials.json` | 0600 | Jira API token + email |
| `state.json` | 0644 | Ticket mirror + activation journal |

---

## Development

```bash
make build            # Build binary
make test             # Unit tests with race detector
make verify-additive  # Enforce additive-only cmux invariant
go vet ./...          # Vet
```

See [docs/smoke-test.md](docs/smoke-test.md) for the manual end-to-end checklist.

---

## Credits

Based on [openkanban](https://github.com/TechDufus/openkanban) by TechDufus. The BubbleTea board view in `internal/ui/` and the git-worktree helpers in `internal/git/` were ported from it.

---

## License

[AGPL-3.0](LICENSE)
