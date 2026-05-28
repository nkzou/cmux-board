package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	internalsync "github.com/nkzou/cmux-board/internal/sync"
	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// pushResultMsg is emitted when the async push goroutine completes.
// It carries a pushResult so the Update loop can handle ok/conflict/error uniformly.
type pushResultMsg struct {
	ticketID     string
	fromColumn   string // column ID before the drag, used for snap-back
	newStatus    string // the target status that was requested
	serverStatus string // server-reported status on conflict (empty on success)
	conflict     bool
	err          error
}

// dropTicket executes the drag-drop column transition for m.dragTicketID → m.dragTargetColumn.
// If dragging to the same column: clears drag state and is a no-op (no Push call).
// Otherwise: optimistically moves the card in the local model, clears drag state,
// and starts the async push goroutine.
//
// The returned tea.Cmd emits a pushResultMsg when the push completes.
// sync.Push is called asynchronously — Update() never blocks.
func (m Model) dropTicket() (Model, tea.Cmd) {
	if !m.dragging {
		return m, nil
	}

	from := m.dragFromColumn
	to := m.dragTargetColumn
	ticketID := m.dragTicketID

	// Clear drag state unconditionally.
	m.dragging = false
	m.dragTicketID = ""
	m.dragFromColumn = ""
	m.dragTargetColumn = ""

	if from == to || to == "" {
		// Same column or no target — no-op.
		return m, nil
	}

	// Resolve the target status name (first StatusID of the target column).
	targetStatus, err := m.resolveTargetStatus(to)
	if err != nil {
		return m.pushToast(fmt.Sprintf("drop failed: %s", err.Error()))
	}

	// Optimistic move: update Status in local model snapshot.
	m = m.optimisticallyMove(ticketID, targetStatus)

	// Start async push goroutine.
	ctx := m.ctx
	store := m.store
	tr := m.tr
	dryRun := m.cfg != nil && m.cfg.DryRun

	return m, func() tea.Msg {
		result, pushErr := internalsync.Push(ctx, store, tr, ticketID, targetStatus, dryRun)
		if pushErr != nil {
			return pushResultMsg{
				ticketID:   ticketID,
				fromColumn: from,
				newStatus:  targetStatus,
				err:        pushErr,
			}
		}
		if result.Conflict || result.DryRun {
			return pushResultMsg{
				ticketID:     ticketID,
				fromColumn:   from,
				newStatus:    targetStatus,
				serverStatus: result.ServerStatus,
				conflict:     true,
			}
		}
		return pushResultMsg{
			ticketID:  ticketID,
			fromColumn: from,
			newStatus: targetStatus,
		}
	}
}

// resolveTargetStatus returns the first StatusID for the given column ID.
// Returns an error when the column is not found or has no StatusIDs.
func (m Model) resolveTargetStatus(columnID string) (string, error) {
	for _, col := range m.board.Columns {
		if col.ID == columnID {
			if len(col.StatusIDs) == 0 {
				return "", fmt.Errorf("column %q has no status IDs configured", columnID)
			}
			return col.StatusIDs[0], nil
		}
	}
	return "", fmt.Errorf("column %q not found in board", columnID)
}

// optimisticallyMove updates Status in the local model snapshot via store.Mutate.
// This gives an immediate UI response before the push result arrives.
// On conflict, the push goroutine will emit a pushResultMsg that triggers snap-back.
func (m Model) optimisticallyMove(ticketID, targetStatus string) Model {
	_ = m.store.Mutate(func(s *state.State) error {
		if t, ok := s.Tickets[ticketID]; ok {
			t.Status = targetStatus
			s.Tickets[ticketID] = t
		}
		return nil
	})
	// Refresh snapshot so the board re-renders with the optimistic position.
	snap, rev := m.store.Snapshot()
	m.snapshot = snap
	m.snapshotRev = rev
	mapped, unmapped := resolveTicketsWithBoard(m.board, snap)
	m.tickets = flattenMapped(m.board, mapped)
	m.unmappedTickets = unmapped
	return m
}

// handlePushResult processes the outcome of a completed push.
// On success: store.Mutate already committed the new status (done by sync.Push).
// On conflict/dry-run: snaps the card back to fromColumn using ServerStatus.
func (m Model) handlePushResult(msg pushResultMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		return m.pushToast("push failed: " + msg.err.Error())
	}
	if msg.conflict {
		// Snap back: restore Status to the server-reported status.
		// This does NOT require a fresh tracker fetch — ServerStatus is carried in the msg.
		_ = m.store.Mutate(func(s *state.State) error {
			if t, ok := s.Tickets[msg.ticketID]; ok {
				t.Status = msg.serverStatus
				s.Tickets[msg.ticketID] = t
			}
			return nil
		})
		snap, rev := m.store.Snapshot()
		m.snapshot = snap
		m.snapshotRev = rev
		mapped, unmapped := resolveTicketsWithBoard(m.board, snap)
		m.tickets = flattenMapped(m.board, mapped)
		m.unmappedTickets = unmapped

		// Emit conflict toast.
		return m.pushToast("conflict: ticket moved by another user; board refreshed")
	}

	// Success: sync.Push already persisted via store.Mutate. Refresh snapshot.
	snap, rev := m.store.Snapshot()
	m.snapshot = snap
	m.snapshotRev = rev
	mapped, unmapped := resolveTicketsWithBoard(m.board, snap)
	m.tickets = flattenMapped(m.board, mapped)
	m.unmappedTickets = unmapped
	return m, nil
}

// pushCmdWith is a tea.Cmd factory that calls push with the given tracker.
// Used in tests to inject a mock tracker into the async push goroutine.
func pushCmdWith(
	ctx context.Context,
	store *state.Store,
	tr tracker.IssueTracker,
	ticketID, fromColumn, targetStatus string,
	dryRun bool,
) tea.Cmd {
	return func() tea.Msg {
		result, err := internalsync.Push(ctx, store, tr, ticketID, targetStatus, dryRun)
		if err != nil {
			return pushResultMsg{ticketID: ticketID, fromColumn: fromColumn, newStatus: targetStatus, err: err}
		}
		if result.Conflict || result.DryRun {
			return pushResultMsg{
				ticketID:     ticketID,
				fromColumn:   fromColumn,
				newStatus:    targetStatus,
				serverStatus: result.ServerStatus,
				conflict:     true,
			}
		}
		return pushResultMsg{ticketID: ticketID, fromColumn: fromColumn, newStatus: targetStatus}
	}
}
