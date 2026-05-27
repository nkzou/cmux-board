package ui

import (
	"github.com/kevin-zou/cmux-board/internal/state"
)

// RepoResolution is the output of resolveRepoAndRoute.
type RepoResolution struct {
	RepoID    string // empty when Cancelled is true or when a picker was opened
	Cancelled bool
	// PickerOpened is true when the function opened ModeRepoPicker (0 or 2+ assigned).
	// Caller should wait for repoPickerSelectedMsg before proceeding.
	PickerOpened bool
}

// resolveRepoAndRoute examines assigned_repo_ids for ticketID and routes the
// Enter key action accordingly. It returns a (Model, RepoResolution) pair so
// callers can open the correct overlay or proceed to activation (T-062).
//
// Routing rules (per FEATURE.md §"resolve_repo_for_activation"):
//
//	0 assigned: open repo picker over all cfg.Repos (first-touch flow).
//	            If cfg.Repos is empty, push a toast and return Cancelled.
//	1 assigned: return that repoID immediately (no overlay).
//	2+ assigned: open repo picker restricted to assigned ids.
func resolveRepoAndRoute(m Model, ticketID string) (Model, RepoResolution) {
	assignedIDs := state.AssignedRepoIDs(m.snapshot, ticketID)

	switch len(assignedIDs) {
	case 0:
		if len(m.cfg.Repos) == 0 {
			m, _ = m.pushToast("no repos registered — run cmux-board repos add")
			return m, RepoResolution{Cancelled: true}
		}
		// First-touch: open repo picker over all registered repos.
		m.repoPicker = newRepoPickerState(m.cfg, ticketID, nil, true)
		m.mode = ModeRepoPicker
		return m, RepoResolution{PickerOpened: true}

	case 1:
		// Single assigned repo: resolve immediately.
		return m, RepoResolution{RepoID: assignedIDs[0]}

	default:
		// Multi-repo: open repo picker restricted to assigned ids.
		m.repoPicker = newRepoPickerState(m.cfg, ticketID, assignedIDs, false)
		m.mode = ModeRepoPicker
		return m, RepoResolution{PickerOpened: true}
	}
}

