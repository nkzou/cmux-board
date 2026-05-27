package sync

import "github.com/nkzou/cmux-board/internal/state"

// Resolve partitions tickets into mapped (column ID → ticket slice) and unmapped.
// A ticket is unmapped when its Status field does not appear in any column's StatusIDs.
// The unmapped slice is non-nil only when len > 0; the UI shows the Unmapped column
// only in that case (E1).
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
	// Build status → column ID lookup.
	statusToCol := make(map[string]string)
	for _, col := range board.Columns {
		for _, sid := range col.StatusIDs {
			statusToCol[sid] = col.ID
		}
	}

	mapped = make(map[string][]state.TicketState)

	for _, t := range tickets {
		// Skip soft-deleted tickets.
		if t.RemovedAt != nil {
			continue
		}
		if colID, ok := statusToCol[t.Status]; ok {
			mapped[colID] = append(mapped[colID], t)
		} else {
			unmapped = append(unmapped, t)
		}
	}

	return mapped, unmapped
}
