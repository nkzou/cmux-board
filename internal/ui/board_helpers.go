package ui

import (
	internalsync "github.com/nkzou/cmux-board/internal/sync"
	"github.com/nkzou/cmux-board/internal/state"
)

// resolveTicketsWithBoard partitions the snapshot's tickets into mapped and unmapped buckets
// via sync.Resolve using the provided board layout.
// Board layout is not stored on State (schema v3); callers must supply the model's board field.
func resolveTicketsWithBoard(board state.BoardSnapshot, snap *state.State) (mapped map[string][]state.TicketState, unmapped []state.TicketState) {
	return internalsync.Resolve(board, snap.Tickets)
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
