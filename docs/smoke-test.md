# cmux-board Smoke Test

Manual end-to-end checklist. Run against a real Jira board, a running cmux instance,
and `claude` CLI version >= 2.1.150. All checkboxes must be ticked for a passing run.
Record the commit SHA and timestamp in the verdict at the bottom.

## Prerequisites

Before starting, confirm all of the following:

- `cmux` is running: `cmux --json identify` exits 0 and prints a JSON blob.
- `claude --version` reports >= 2.1.150.
- You have a Jira Cloud board and a token with `read:jira-work`, `write:jira-work` scopes.
- Binary is built: `make build && ls -la cmux-board`.
- Config directory does NOT exist yet (fresh run):
  `ls ~/.config/cmux-board/` should say "No such file or directory".
  If it exists, run `cmux-board init --reset` first.
- One git repository (a real `git clone`) is available at a known path.

---

## Phase 1 — init wizard

- [ ] **1.1** Run `./cmux-board init`.
  Expected: adapter pick menu shows "Jira (Cloud)".

- [ ] **1.2** Select Jira. Enter your Jira site (e.g. `yourorg.atlassian.net`). Enter your
  API token at the masked prompt (or via `--api-token-stdin` to pipe from a password manager).
  Expected: wizard prints your display name and email from WhoAmI.

- [ ] **1.3** Pick a board from the numbered list.
  Expected: board name echoed back.

- [ ] **1.4** Register one repo when prompted. Enter the absolute path to a git clone.
  Expected: derived slug is printed; `./cmux-board repos list` shows it.

- [ ] **1.5** Confirm to write files.
  Expected: wizard prints `config.json written`, `credentials.json written`, `state.json written`
  and shows the Dock JSON snippet for `~/.config/cmux/dock.json`.

- [ ] **1.6** Verify mode bits:
  ```
  stat -f "%A %N" ~/.config/cmux-board/credentials.json \
                  ~/.config/cmux-board/config.json \
                  ~/.config/cmux-board/state.json
  stat -f "%A %N" ~/.config/cmux-board/
  ```
  Expected: `600` for `credentials.json`, `644` for `config.json` and `state.json`, `700` for
  the directory.

- [ ] **1.7** Verify schema version:
  ```
  python3 -c "import json; d=json.load(open('/Users/$USER/.config/cmux-board/config.json')); print(d['schema_version'])"
  ```
  Expected: `2`.

---

## Phase 2 — dock startup and board render

- [ ] **2.1** Run `./cmux-board dock`.
  Expected: no pre-flight errors; board renders within 2 seconds; ticket cards appear.

- [ ] **2.2** Confirm the `tracker` status pill in the header is green (healthy).

- [ ] **2.3** Confirm board columns match the Jira board column configuration.

- [ ] **2.4** If any ticket has a status not mapped to any column, confirm that a synthetic
  **Unmapped** column appears on the right with the ticket showing a `?` badge. If all
  statuses are mapped, skip this step and mark it N/A. (E1)

- [ ] **2.5** After the first poll fires (within 10 seconds), verify `state.json` under
  `"tickets"` contains the fetched tickets:
  ```
  python3 -c "import json; d=json.load(open('/Users/$USER/.config/cmux-board/state.json')); print(list(d['tickets'].keys())[:3])"
  ```

---

## Phase 3 — ticket activation (0 activations, create)

- [ ] **3.1** Navigate to a ticket card. Press `Enter`.
  Expected (if the ticket has no `assigned_repo_ids`): a repo picker appears listing the repo
  registered in Phase 1.4. (F-MR2)

- [ ] **3.2** Select the repo from the picker.
  Expected: activation begins; a toast or status update appears.

- [ ] **3.3** After activation completes, verify all three resources were created:
  ```
  # Worktree exists under worktree base dir (adjust path):
  ls "$HOME/.code/worktrees/" | grep <TICKET_KEY>

  # state.json shows complete: true
  python3 -c "
  import json
  d = json.load(open('/Users/$USER/.config/cmux-board/state.json'))
  for k, entries in d.get('activations', {}).items():
      for e in entries:
          print(k, e.get('act_id_short'), e.get('complete'), e.get('step'))
  "

  # cmux workspace exists
  cmux list-workspaces --json | python3 -c "import json,sys; ws=json.load(sys.stdin); [print(w['title']) for w in ws]"
  ```
  Expected: worktree directory exists with `act_id_short` in the name; `complete: true`;
  a workspace with the ticket key in its title.

- [ ] **3.4** Confirm the cmux workspace that opened has two panes:
  - Left pane: running `claude attach <short_id>` (agent view).
  - Right pane: a shell with `PWD` set to the worktree path.

- [ ] **3.5** Measure activate-to-focus latency. From pressing Enter to the cmux workspace
  becoming visible should be under 500ms on a healthy system. (R11)
  Observed latency: ______ms.

---

## Phase 4 — focus flow (1 activation, focus existing)

- [ ] **4.1** Switch back to the `cmux-board dock` pane (or window). Navigate to the same
  ticket. Press `Enter`.
  Expected: `cmux focus-pane` is called; the existing workspace comes to the front. No new
  workspace is created. (F14)

- [ ] **4.2** Confirm no new workspace was created:
  ```
  cmux list-workspaces --json | python3 -c "import json,sys; ws=json.load(sys.stdin); print(len([w for w in ws if '<TICKET_KEY>' in w['title']]))"
  ```
  Expected: `1`.

---

## Phase 5 — orphan detection and respawn (r key)

- [ ] **5.1** In a terminal, simulate a stale session:
  ```
  claude rm <SHORT_ID_FROM_STATE>
  ```

- [ ] **5.2** In the dock, press `Enter` on the same ticket.
  Expected: picker opens with an `[orphan]` glyph next to the activation. (E7)

- [ ] **5.3** Press `r` in the picker.
  Expected: a new `claude --bg` session is spawned; `state.json` shows an updated
  `claude_short_id`; `claude_orphan` field is absent or false.

- [ ] **5.4** Confirm the new session's `--name` still contains the same `act_id_short`
  (the same ULID is reused, not a new one):
  ```
  claude agents --json | python3 -c "import json,sys; agents=json.load(sys.stdin); [print(a.get('name')) for a in agents if '<ACT_ID_SHORT>' in a.get('name','')]"
  ```

---

## Phase 6 — assignment editor (a key)

- [ ] **6.1** Register a second repo:
  ```
  ./cmux-board repos add <PATH_TO_SECOND_REPO>
  ./cmux-board repos list
  ```
  Expected: second repo appears in the list.

- [ ] **6.2** Navigate to the ticket. Press `a`.
  Expected: assignment editor overlay appears with both repos listed; the repo from Phase 3 is
  pre-checked. (F-MR4)

- [ ] **6.3** Press `space` to toggle the second repo. Press `Enter`.
  Expected: `state.json` `assigned_repo_ids` for the ticket now contains both repo IDs.
  ```
  python3 -c "import json; d=json.load(open('/Users/$USER/.config/cmux-board/state.json')); print(d['tickets']['<TICKET_KEY>']['assigned_repo_ids'])"
  ```

- [ ] **6.4** Press `Enter` on the ticket.
  Expected: a per-repo picker appears asking which repo to work in. (F-MR3)

---

## Phase 7 — second-repo activation

- [ ] **7.1** In the per-repo picker from Phase 6.4, select the second repo.
  Expected: a new activation entry is created with `repo_id` equal to the second repo's ID;
  a new worktree is created under the second repo's base path.

- [ ] **7.2** Verify two activations with different repo IDs:
  ```
  python3 -c "
  import json
  d = json.load(open('/Users/$USER/.config/cmux-board/state.json'))
  for e in d.get('activations', {}).get('<TICKET_KEY>', []):
      print(e.get('repo_id'), e.get('act_id_short'), e.get('complete'))
  "
  ```
  Expected: two lines with different `repo_id` values, both `complete: True`.

---

## Phase 8 — card move and OCC

- [ ] **8.1** Navigate to a different ticket (not the one activated above). Press `l` to move
  it right to the next column.
  Expected: immediate push; tracker pill stays green; card appears in the new column;
  `last_known_status` in `state.json` reflects the new status.

- [ ] **8.2** Simulate an OCC conflict: in a browser, change the same ticket's status in Jira
  to a different column. Immediately move the card in the dock via `l`.
  Expected: a non-modal toast appears ("conflict -- snapped back"); card returns to the
  column matching Jira's current status. No duplicate transitions. (F12, E3)

---

## Phase 9 — clean shutdown

- [ ] **9.1** Press `q` to quit the dock.
  Expected: exit code 0; `state.json` modification time is current (written on shutdown).
  ```
  echo $?   # should be 0
  stat -f "%Sm" ~/.config/cmux-board/state.json
  ```

- [ ] **9.2** Restart `./cmux-board dock`.
  Expected: board renders from the persisted `state.json`; no reconciliation warnings for the
  two completed activations from Phases 3 and 7; status pills are green after first poll.

---

## Pass Criteria

All steps must have `[x]`. Any `[ ]` at the end of a run is a failure. File a bug for each
failure as a discovered bundle before marking T-086 complete.

## Run Record

| Field | Value |
|---|---|
| Date | |
| Commit SHA | |
| `cmux --version` | |
| `claude --version` | |
| Jira site | |
| Board name | |
| Activate-to-focus (Phase 3.5) | ms |
| Failures | none / list below |
| Verdict | PASS / FAIL |

Failures (if any):

1.
2.
