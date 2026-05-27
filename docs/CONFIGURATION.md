# Configuration

cmux-board stores all config and state under `~/.config/cmux-board/`.

## File layout

```
~/.config/cmux-board/
  config.json        mode 0644 -- user preferences and repo registry
  state.json         mode 0644 -- ticket cache, activations, board snapshot
  credentials.json   mode 0600 -- API tokens (mode enforced; refused if != 0600)
```

The binary refuses to start if `credentials.json` has permissions other than 0600.

Schema v1 is refused on read. Both config.json and state.json must be schema_version 2.
Run `cmux-board init` to generate a seed config.

## config.json

Authoritative schema: `internal/config/config.go`.

```json
{
  "schema_version": 2,
  "adapter": "jira",
  "adapter_config": {
    "site_url": "https://yourorg.atlassian.net",
    "board_id": "42"
  },
  "board_id": "42",
  "repos": {
    "my-service": {
      "id": "my-service",
      "name": "my-service",
      "path": "/home/user/code/my-service",
      "default_branch": "main"
    }
  },
  "poll_interval_seconds": 60,
  "worktree_base_dir": "/home/user/worktrees",
  "claude": {
    "model": "claude-opus-4-5",
    "permission_mode": "bypassPermissions",
    "starter_prompt": ""
  },
  "dry_run": false
}
```

When `dry_run` is true, the binary skips all `TransitionStatus` calls to the tracker.
The poller continues to read tickets normally. Card-move attempts return a non-modal
toast and snap the card back to its prior column.

## state.json

Authoritative schema: `internal/state/state.go`.

```json
{
  "schema_version": 2,
  "tickets": {
    "PROJ-42": {
      "key": "PROJ-42",
      "summary": "Fix login bug",
      "status": "In Progress",
      "last_known_status": "To Do",
      "assigned_repo_ids": ["my-service"],
      "removed_at": null
    }
  },
  "activations": {
    "PROJ-42": [
      {
        "activation_id": "01JXXXXXXXXXXXXXXXXXXXXXXXX",
        "act_id_short": "0abc1234",
        "repo_id": "my-service",
        "ticket_id": "PROJ-42",
        "approach_name": "main",
        "worktree_path": "/home/user/worktrees/my-service-PROJ-42-main-0abc1234",
        "branch_name": "proj-42-main-0abc1234",
        "claude_name": "cmux-board:PROJ-42:0abc1234",
        "cmux_name": "PROJ-42 [0abc1234]",
        "step": "cmux_created",
        "complete": true,
        "created_at": "2026-01-01T00:00:00Z"
      }
    ]
  }
}
```

`activation_id` is a full ULID. `act_id_short` is the first 8 Crockford-base32
characters of the ULID, lowercased. The step field tracks journal progress:
`started` -> `worktree_created` -> `claude_started` -> `cmux_created`.

`removed_at` on a ticket is set when the ticket leaves the polled set but is kept
in state for historical resolution of activations.

## credentials.json

Authoritative schema: `internal/config/credentials.go`.

```json
{
  "schema_version": 2,
  "adapters": {
    "jira": {
      "email": "user@example.com",
      "api_token": "ATATT...",
      "site_url": "https://yourorg.atlassian.net"
    }
  }
}
```

The raw `api_token` string is the only value registered with `secretsink.Register`.
The email field is not registered. `secretsink` redacts registered values from all
log output before any bytes reach disk or stdout.

## Init wizard

`cmux-board init` writes a seed `config.json` and `credentials.json` interactively.
It does not overwrite existing files unless `--force-reset` is passed.
