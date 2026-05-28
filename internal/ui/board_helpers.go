package ui

import (
	internalsync "github.com/nkzou/cmux-board/internal/sync"
	"github.com/nkzou/cmux-board/internal/state"
)

// resolveTickets partitions the snapshot's tickets into mapped and unmapped buckets
// via sync.Resolve. Removed tickets are excluded by sync.Resolve.
func resolveTickets(snap *state.State) (mapped map[string][]state.TicketState, unmapped []state.TicketState) {
	return internalsync.Resolve(snap.Board, snap.Tickets)
}

// flattenMapped flattens the mapped bucket into a single slice ordered by board column.
// Tickets within a column retain the order returned by sync.Resolve.
func flattenMapped(board state.BoardSnapshot, mapped map[string][]state.TicketState) []state.TicketState {
	var out []state.TicketState
	for _, col := range board.Columns {
		out = append(out, mapped[col.ID]...)
	}
	return out
}
