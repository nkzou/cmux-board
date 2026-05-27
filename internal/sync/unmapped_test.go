package sync

import (
	"testing"
	"time"

	"github.com/nkzou/cmux-board/internal/state"
)

func makeBoard(colID string, statusIDs []string) state.BoardSnapshot {
	return state.BoardSnapshot{
		Columns: []state.ColumnSnapshot{
			{ID: colID, Name: colID, StatusIDs: statusIDs},
		},
	}
}

// TC-1: all tickets mapped to the correct column.
func TestResolve_AllMapped(t *testing.T) {
	board := makeBoard("in-progress", []string{"In Progress"})
	tickets := map[string]state.TicketState{
		"PROJ-1": {Key: "PROJ-1", Status: "In Progress"},
	}
	mapped, unmapped := Resolve(board, tickets)

	if len(unmapped) != 0 {
		t.Errorf("TC-1: want 0 unmapped, got %d", len(unmapped))
	}
	col, ok := mapped["in-progress"]
	if !ok {
		t.Fatal("TC-1: mapped[\"in-progress\"] not found")
	}
	if len(col) != 1 || col[0].Key != "PROJ-1" {
		t.Errorf("TC-1: mapped[\"in-progress\"]: want [PROJ-1], got %v", col)
	}
}

// TC-2: one ticket mapped, one unmapped (E1).
func TestResolve_OneUnmapped(t *testing.T) {
	board := makeBoard("in-progress", []string{"In Progress"})
	tickets := map[string]state.TicketState{
		"PROJ-1": {Key: "PROJ-1", Status: "In Progress"},
		"PROJ-2": {Key: "PROJ-2", Status: "Weird State"},
	}
	mapped, unmapped := Resolve(board, tickets)

	col := mapped["in-progress"]
	if len(col) != 1 || col[0].Key != "PROJ-1" {
		t.Errorf("TC-2: mapped[\"in-progress\"]: want [PROJ-1], got %v", col)
	}
	if len(unmapped) != 1 || unmapped[0].Key != "PROJ-2" {
		t.Errorf("TC-2: unmapped: want [PROJ-2], got %v", unmapped)
	}
}

// TC-3: all tickets unmapped.
func TestResolve_AllUnmapped(t *testing.T) {
	board := makeBoard("done", []string{"Done"})
	tickets := map[string]state.TicketState{
		"PROJ-1": {Key: "PROJ-1", Status: "Open"},
		"PROJ-2": {Key: "PROJ-2", Status: "In Review"},
	}
	mapped, unmapped := Resolve(board, tickets)

	for k, v := range mapped {
		if len(v) > 0 {
			t.Errorf("TC-3: mapped[%q] should be empty, got %v", k, v)
		}
	}
	if len(unmapped) != 2 {
		t.Errorf("TC-3: want 2 unmapped, got %d", len(unmapped))
	}
}

// TC-4: removed tickets excluded from both outputs.
func TestResolve_RemovedTicketsExcluded(t *testing.T) {
	board := makeBoard("in-progress", []string{"In Progress"})
	removedAt := time.Now()
	tickets := map[string]state.TicketState{
		"PROJ-42": {Key: "PROJ-42", Status: "In Progress", RemovedAt: &removedAt},
	}
	mapped, unmapped := Resolve(board, tickets)

	for _, v := range mapped {
		for _, ticket := range v {
			if ticket.Key == "PROJ-42" {
				t.Error("TC-4: PROJ-42 (removed) must not appear in mapped")
			}
		}
	}
	for _, ticket := range unmapped {
		if ticket.Key == "PROJ-42" {
			t.Error("TC-4: PROJ-42 (removed) must not appear in unmapped")
		}
	}
}

// TC-5: empty board and empty tickets produce empty outputs without panic.
func TestResolve_EmptyInputs(t *testing.T) {
	mapped, unmapped := Resolve(state.BoardSnapshot{}, nil)
	if len(mapped) != 0 {
		t.Errorf("TC-5: mapped should be empty, got %v", mapped)
	}
	if len(unmapped) != 0 {
		t.Errorf("TC-5: unmapped should be empty, got %v", unmapped)
	}
}

// TC-6: multiple tickets per column.
func TestResolve_MultipleTicketsPerColumn(t *testing.T) {
	board := makeBoard("in-progress", []string{"In Progress"})
	tickets := map[string]state.TicketState{
		"PROJ-1": {Key: "PROJ-1", Status: "In Progress"},
		"PROJ-2": {Key: "PROJ-2", Status: "In Progress"},
	}
	mapped, _ := Resolve(board, tickets)

	col := mapped["in-progress"]
	if len(col) != 2 {
		t.Errorf("TC-6: want 2 tickets in column, got %d", len(col))
	}
}

// TC-7: case-insensitive status matching — acli may return names with different
// capitalisation than what the user typed in the wizard.
func TestResolve_CaseInsensitiveStatus(t *testing.T) {
	// Column configured with "In Progress" (title case).
	board := makeBoard("in-progress", []string{"In Progress"})
	tickets := map[string]state.TicketState{
		// Ticket status arrives lowercase from some acli version.
		"PROJ-1": {Key: "PROJ-1", Status: "in progress"},
		// Ticket status arrives all-caps.
		"PROJ-2": {Key: "PROJ-2", Status: "IN PROGRESS"},
		// Exact match still works.
		"PROJ-3": {Key: "PROJ-3", Status: "In Progress"},
	}
	mapped, unmapped := Resolve(board, tickets)

	if len(unmapped) != 0 {
		t.Errorf("TC-7: want 0 unmapped, got %d: %v", len(unmapped), unmapped)
	}
	col := mapped["in-progress"]
	if len(col) != 3 {
		t.Errorf("TC-7: want 3 tickets in column, got %d: %v", len(col), col)
	}
}
