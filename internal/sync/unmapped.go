package sync

import (
	"sort"
	"strconv"
	"strings"

	"github.com/nkzou/cmux-board/internal/state"
)

// Resolve partitions tickets into mapped (column ID → ticket slice) and unmapped.
// A ticket is unmapped when its Status field does not appear in any column's StatusIDs.
// The unmapped slice is non-nil only when len > 0; the UI shows the Unmapped column
// only in that case (E1).
//
// Status matching is case-insensitive. This is required for the acli backend where
// column StatusIDs store status names (not numeric IDs) and acli may return names
// with different capitalisation across API versions.
//
// Removed tickets (non-nil removed_at) are excluded from both outputs — they are
// hidden from the kanban view entirely.
//
// Parameters:
//
//	board   — current board snapshot from state (columns + StatusIDs per column).
//	tickets — map of all TicketState entries (use store.Snapshot().Tickets).
//
// Returns copies of TicketState values; callers must not mutate the returned values
// to avoid races (the original state is owned by Store).
func Resolve(
	board state.BoardSnapshot,
	tickets map[string]state.TicketState,
) (mapped map[string][]state.TicketState, unmapped []state.TicketState) {
	// Build lowercase-status → column ID lookup for case-insensitive matching.
	statusToCol := make(map[string]string)
	for _, col := range board.Columns {
		for _, sid := range col.StatusIDs {
			statusToCol[strings.ToLower(sid)] = col.ID
		}
	}

	mapped = make(map[string][]state.TicketState)

	for _, t := range tickets {
		if colID, ok := statusToCol[strings.ToLower(t.Status)]; ok {
			mapped[colID] = append(mapped[colID], t)
		} else {
			unmapped = append(unmapped, t)
		}
	}

	// Sort each column (and the unmapped bucket) by Key so render order is stable
	// across polls. Map iteration above is non-deterministic; without this, tickets
	// would visibly reshuffle on every refresh.
	for colID := range mapped {
		sortByKey(mapped[colID])
	}
	sortByKey(unmapped)

	return mapped, unmapped
}

// sortByKey sorts tickets in place by Key using natural ordering so that the
// numeric suffix is compared as a number (PROJ-2 before PROJ-10), not as a
// string (PROJ-10 before PROJ-2).
func sortByKey(ts []state.TicketState) {
	sort.Slice(ts, func(i, j int) bool {
		return lessKey(ts[i].Key, ts[j].Key)
	})
}

// lessKey compares two Jira-style keys ("PROJ-42") with the trailing integer
// compared numerically. Falls back to a plain lexicographic compare when either
// key has no parseable numeric suffix.
func lessKey(a, b string) bool {
	ap, an, aOK := splitKey(a)
	bp, bn, bOK := splitKey(b)
	if aOK && bOK && ap == bp {
		return an < bn
	}
	return a < b
}

// splitKey returns the prefix before the last '-' and the integer after it.
// Returns ok=false when the suffix is missing or not an integer.
func splitKey(k string) (prefix string, num int, ok bool) {
	i := strings.LastIndex(k, "-")
	if i < 0 || i == len(k)-1 {
		return "", 0, false
	}
	n, err := strconv.Atoi(k[i+1:])
	if err != nil {
		return "", 0, false
	}
	return k[:i], n, true
}
