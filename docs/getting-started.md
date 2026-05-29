# Getting started with cmux-board

This guide walks you from zero to a running board with your first agent activated.

---

## Prerequisites

Install and verify each tool before continuing:

```bash
# cmux
cmux --version

# Claude Code (minimum v2.1.150)
claude --version

# acli authenticated to Jira Cloud
acli jira auth status
```

If `acli jira auth status` returns an error, run `acli jira auth login` and complete the OAuth flow before proceeding.

---

## 1. Install

```bash
git clone https://github.com/nkzou/cmux-board
cd cmux-board
make build
mv cmux-board /usr/local/bin/
```

Verify:

```bash
cmux-board --help
```

---

## 2. Run the init wizard

```bash
cmux-board init
```

The wizard runs five steps:

**Step 1: Adapter**
Select `jira` (the only option currently).

**Step 2: Authentication**
The wizard calls `acli jira auth status` to verify you are authenticated and to resolve your Jira site URL. No credentials are prompted here; the wizard reads from acli's stored auth.

**Step 3: Board selection**
A list of boards from your Jira site is displayed. Select the board you work on daily. If you know the board ID already, you can skip this step:

```bash
cmux-board init --board-id 42
```

**Step 4: Column configuration**
The wizard discovers all statuses on the board and prompts you to group them into columns. A typical setup:

```
To Do       -> To Do, Backlog
In Progress -> In Progress, In Review
Done        -> Done, Closed
```

Tickets with statuses not mapped to any column appear in an "Unmapped" column on the right and cannot be moved.

**Step 5: Repo registration**
Register every git repo you want to create worktrees in. For each repo you enter a path; the wizard validates it is a real git repository and resolves the default branch.

```
Path: /home/user/code/my-service
  -> resolved default branch: main
  -> repo ID: my-service
```

You can add more repos later:

```bash
cmux-board repos add /path/to/another-repo
```

**Finalization**
The wizard writes three files:

| File | Location |
|------|----------|
| `config.json` | `~/.config/cmux-board/config.json` |
| `credentials.json` | `~/.config/cmux-board/credentials.json` (mode 0600) |
| `state.json` | `~/.config/cmux-board/state.json` |

---

## 3. Start the board

```bash
cmux-board dock
```

The startup sequence runs 10 preflight checks and then opens the TUI. On first run it fetches all tickets from your Jira board and displays them in columns.

If a preflight check fails, the binary exits with an actionable error message. Common causes:

| Error | Fix |
|-------|-----|
| `credentials.json` permissions are not 0600 | `chmod 600 ~/.config/cmux-board/credentials.json` |
| cmux not available | Start cmux: `cmux start` |
| claude not available or too old | Update Claude Code: `claude update` |
| schema v1 detected | Run `cmux-board init --reset` to regenerate config |

---

## 4. Navigate the board

The board opens with vim-style navigation:

- `h` / `l`: move left and right across columns.
- `j` / `k`: move down and up within a column.
- Arrow keys also work if you prefer them.

The selected ticket is highlighted. Press `?` to see the full keybinding reference at any time.

---

## 5. Assign a repo to a ticket

Before activating a ticket, assign at least one repo to it. Press `a` on the selected ticket to open the assignment editor.

Use `j` / `k` to navigate the repo list and `Space` to toggle each repo. Press `Enter` to commit.

If you only have one repo registered, the assignment editor still lets you confirm which repo to use for that ticket. Tickets with no assigned repo cannot be activated.

---

## 6. Activate your first ticket

With a repo assigned, press `Enter` on the ticket. cmux-board:

1. Creates a git worktree branched from the repo's default branch.
2. Spawns `claude --bg` in that worktree with a prompt describing the ticket.
3. Opens a new cmux workspace with the agent view on the left and a shell on the right.
4. Focuses the agent pane.

You should see the cmux workspace switch to the new session automatically.

The ticket card gains a dot indicator showing one active activation.

---

## 7. Focus an existing activation

Press `Enter` again on an activated ticket to focus the existing workspace. If the ticket has exactly one activation, it focuses directly. If it has two or more, a picker opens listing each by approach name and ID.

From the picker:

- `Enter`: focus the selected activation.
- `r`: respawn (re-open) a workspace whose cmux session was closed.
- `d`: delete the activation record.
- `n`: start a new approach from the picker.
- `Esc`: cancel.

---

## 8. Multiple approaches per ticket

Press `N` (capital) to start a second approach on the same ticket without going through the picker. A text input opens prompting for an approach name (e.g. `fast-path`, `refactor-first`).

Each approach gets its own worktree and its own Claude session. They run independently.

Press `m` at any time to open the activation picker and manage all active approaches for the current ticket.

---

## 9. Move a ticket across columns

Use `h` / `l` to move a ticket left or right. cmux-board immediately calls `TransitionStatus` on the Jira issue. If Jira rejects the transition (wrong workflow path, permission error, etc.), the card snaps back and a toast appears with the error.

---

## 10. Import and create tickets

**Import a Jira ticket by key**: press `i`, type the issue key (e.g. `PROJ-42`), and press `Enter`. The ticket is fetched from Jira and added to the board.

**Create a local ticket**: press `c`, type a summary, and press `Enter`. Local tickets are not synced to Jira. They appear on the board with a `local` source tag and can be activated just like Jira tickets.

Press `s` on a local ticket to cycle its status through the configured column statuses.

Press `x` to remove a ticket from the board. This only removes it from the local state; the Jira issue is not deleted.

---

## 11. Background sync

The poller runs every 60 seconds by default (configurable via `poll_interval_seconds` in `config.json`). It refreshes status, summary, priority, labels, and assignee for all Jira-sourced tickets. It never inserts or deletes tickets automatically.

The status bar at the bottom shows:

- Last successful poll time.
- Poll error type (`auth` or `transient`) with backoff timer if failing.
- cmux and claude health indicators.

---

## Next steps

- Add more repos: `cmux-board repos add <path>`
- Customize the starter prompt: edit `claude.starter_prompt` in `config.json` using the template variables listed in the README.
- Set `dry_run: true` in `config.json` to observe Jira sync without writing any transitions.
- Run `cmux-board dock --debug` to diagnose mouse event pass-through in multiplexer environments.

See [CONFIGURATION.md](CONFIGURATION.md) for the full config and state schema reference.
