# UI Design

cmux-board renders inside a dedicated cmux workspace pane. The pane width is managed
by cmux; the BubbleTea model receives the terminal dimensions from cmux and renders
within them. There are no fullscreen modes -- the board occupies one pane.

## Board layout

The board shows ticket columns side by side. Each column has a header (status name)
and a scrollable list of ticket cards. The active card is highlighted. Column width
adapts to the available pane width divided by the number of columns.

An `Unmapped` column is appended when any ticket's status does not map to any of the
columns returned by the tracker's `Capabilities`. Tickets in `Unmapped` are read-only.

## Card layout

Each card shows: title (truncated to fit), status pill, repo assignment pills
(one per `assigned_repo_ids` entry), and an activation indicator dot when one or more
activations exist.

## Modes

Defined in `internal/ui/modes.go`. The BubbleTea `Update` function dispatches on
the current mode:

- `ModeNormal` -- board navigation, vim-style hjkl + enter
- `ModeFilter` -- type to filter visible tickets by summary substring
- `ModeRepoPicker` -- overlay for selecting an existing activation or creating a new approach
- `ModeAssignmentEditor` -- per-ticket multi-select overlay for assigning repos

## Keymap

Full key binding table is in `internal/ui/keymap.go`. Constants are exported and
referenced by all handler files -- bare string literals in switch cases are forbidden.

## Overlays

- `Picker` (`ModeRepoPicker`) -- activation selector; shows existing activations with
  focus/new/respawn/delete actions.
- `AssignmentEditor` (`ModeAssignmentEditor`) -- per-ticket repo multi-select;
  space to toggle, enter to commit.
- `Toast` -- non-modal notification bar at the bottom. All confirmations and error
  messages use toast; there are no blocking modal prompts.
