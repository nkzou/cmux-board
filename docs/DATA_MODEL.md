# Data Model

Authoritative schemas live in `internal/state/state.go` and `internal/config/config.go`.
This document summarizes the key types and their invariants.

## Ticket

`TicketState` in `internal/state/state.go`.

Tickets are poll-pulled from the tracker. They are never deleted from state. When a
ticket leaves the polled set, `removed_at` is set to the removal timestamp; the entry
is kept for historical resolution of activations.

Tracker-owned fields (`summary`, `status`, `last_known_status`, `assignee_id`,
`labels`, `priority`, `url`, `updated_at`, `raw`) are overwritten by `MergePulledTickets`
on each poll.

Local-only fields (`assigned_repo_ids`, `removed_at`) are preserved across polls and
never overwritten by tracker data.

## Activation

`ActivationEntry` in `internal/state/state.go`.

One entry per (ticket, repo, approach) activation. Fields are journaled incrementally
before each side effect.

Key fields:

- `activation_id` -- full ULID (canonical durable key)
- `act_id_short` -- first 8 Crockford-base32 characters of the ULID, lowercased.
  Regex: `^[0-9a-hjkmnp-tv-z]{8}$`. Used as the human-visible short ID in workspace
  names, claude session names, and reconciliation substring matching.
- `step` -- journal progress: `started` -> `worktree_created` -> `claude_started` -> `cmux_created`
- `complete` -- true only after all three side effects are journaled

## Repo

`RepoEntry` in `internal/config/config.go`. Registered via `cmux-board repos add`.
Identified by `repo_id` (a lowercase slug derived from the directory name). The
authoritative `repo_id` derivation is in `internal/config/repo_id.go`.

## MergePulledTickets contract

`MergePulledTickets` in `internal/sync/` is the only function that writes the
`Tickets` map. It overwrites tracker-owned fields and preserves local-only fields.
No other code path modifies `state.Tickets` directly -- this is enforced by
`scripts/check_additive_only.sh` arm 4.

## Store

`Store` in `internal/state/store.go`.

```
Open(path string) (*Store, error)
Mutate(fn func(*State) error) error
Snapshot() (*State, uint64)
```

`Mutate` is the only write path. It acquires a mutex, calls `fn` on a writable copy of
the state, persists atomically on success, and increments the revision counter.
There is no exported `Save` function -- direct persistence is intentionally not exposed.

`Snapshot` returns a deep-copy of the current state plus its revision number.
Callers that need a consistent read without mutation use `Snapshot`.
