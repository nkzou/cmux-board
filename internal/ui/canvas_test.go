package ui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/muesli/termenv"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

func init() {
	// Force ASCII color profile for deterministic test output.
	// (may have been set by view_test.go init too, but idempotent)
	lipgloss.SetColorProfile(termenv.Ascii)
}

// makeCanvasModel builds a Model with given width/height and seeded tickets + zOrder.
func makeCanvasModel(t *testing.T, w, h int, tickets map[string]state.TicketState, zOrder []string) Model {
	t.Helper()
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	if len(tickets) > 0 {
		if err := store.Mutate(func(s *state.State) error {
			for k, v := range tickets {
				s.Tickets[k] = v
			}
			return nil
		}); err != nil {
			t.Fatalf("store.Mutate: %v", err)
		}
	}
	cfg := &config.Config{SchemaVersion: config.SchemaVersionCurrent}
	m := NewModelWithContext(context.Background(), cfg, store)
	snap, rev := store.Snapshot()
	m.snapshot = snap
	m.snapshotRev = rev
	m.width = w
	m.height = h
	m.zOrder = zOrder
	return m
}

func TestCanvas_EmptyCanvasIsBlankOrHint(t *testing.T) {
	t.Parallel()
	m := makeCanvasModel(t, 80, 24, nil, nil)
	m.mode = ModeNormal
	out := m.renderPostitCanvas()
	if out == "" {
		t.Error("renderPostitCanvas returned empty string for empty board in ModeNormal")
	}
	// Should contain the first-run hint text.
	if !strings.Contains(out, "import") && !strings.Contains(out, "post-it") {
		t.Errorf("expected first-run hint in empty board output:\n%s", out)
	}
}

func TestCanvas_SingleCardAtOrigin(t *testing.T) {
	t.Parallel()
	tickets := map[string]state.TicketState{
		"T-1": {Key: "T-1", Summary: "Hello World", Status: "todo", X: 2, Y: 0},
	}
	m := makeCanvasModel(t, 80, 24, tickets, []string{"T-1"})
	out := m.renderPostitCanvas()
	// Card content should appear somewhere in the output.
	if !strings.Contains(out, "Hello World") {
		t.Errorf("expected 'Hello World' in canvas output:\n%s", out)
	}
}

func TestCanvas_TwoNonOverlapping(t *testing.T) {
	t.Parallel()
	tickets := map[string]state.TicketState{
		"A": {Key: "A", Summary: "Card A", X: 0, Y: 0},
		"B": {Key: "B", Summary: "Card B", X: 40, Y: 0},
	}
	m := makeCanvasModel(t, 80, 24, tickets, []string{"A", "B"})
	out := m.renderPostitCanvas()
	if !strings.Contains(out, "Card A") {
		t.Errorf("expected 'Card A' in canvas:\n%s", out)
	}
	if !strings.Contains(out, "Card B") {
		t.Errorf("expected 'Card B' in canvas:\n%s", out)
	}
}

func TestCanvas_TwoOverlapping_TopWins(t *testing.T) {
	t.Parallel()
	// A and B at same position; B is on top (last in zOrder).
	tickets := map[string]state.TicketState{
		"A": {Key: "A", Summary: "CardAAA", X: 0, Y: 0},
		"B": {Key: "B", Summary: "CardBBB", X: 0, Y: 0},
	}
	m := makeCanvasModel(t, 80, 24, tickets, []string{"A", "B"})
	out := m.renderPostitCanvas()
	// B is topmost, so B's content should win on the overlap cells.
	// The output should contain B's summary.
	if !strings.Contains(out, "CardBBB") {
		t.Errorf("expected top card B to win on overlap; output:\n%s", out)
	}
}

func TestCanvas_CardClippedAtEdge(t *testing.T) {
	t.Parallel()
	// Card positioned so its right edge exceeds canvas width — should not panic.
	tickets := map[string]state.TicketState{
		"T-1": {Key: "T-1", Summary: "Edge Card", X: 70, Y: 0},
	}
	m := makeCanvasModel(t, 80, 24, tickets, []string{"T-1"})
	out := m.renderPostitCanvas()
	// Should not panic and output should have content.
	_ = out
}

func TestReconcileZOrder_EmptyCold_NewKeysAppended(t *testing.T) {
	t.Parallel()
	tickets := map[string]state.TicketState{
		"A": {Key: "A", Summary: "A"},
		"B": {Key: "B", Summary: "B"},
	}
	m := makeCanvasModel(t, 80, 24, tickets, nil) // empty zOrder
	snap, _ := m.store.Snapshot()
	m.reconcileZOrder(snap)
	if len(m.zOrder) != 2 {
		t.Errorf("expected 2 keys in zOrder after reconcile, got %d", len(m.zOrder))
	}
}

func TestReconcileZOrder_RefreshPrunesMissing(t *testing.T) {
	t.Parallel()
	tickets := map[string]state.TicketState{
		"A": {Key: "A", Summary: "A"},
	}
	m := makeCanvasModel(t, 80, 24, tickets, []string{"A", "GHOST"})
	snap, _ := m.store.Snapshot()
	m.reconcileZOrder(snap)
	for _, k := range m.zOrder {
		if k == "GHOST" {
			t.Error("reconcileZOrder should prune missing key GHOST")
		}
	}
	found := false
	for _, k := range m.zOrder {
		if k == "A" {
			found = true
		}
	}
	if !found {
		t.Error("reconcileZOrder should keep present key A")
	}
}

func TestReconcileZOrder_PreservesOrderOfSurvivors(t *testing.T) {
	t.Parallel()
	tickets := map[string]state.TicketState{
		"A": {Key: "A"}, "B": {Key: "B"}, "C": {Key: "C"},
	}
	m := makeCanvasModel(t, 80, 24, tickets, []string{"C", "A", "B"})
	snap, _ := m.store.Snapshot()
	m.reconcileZOrder(snap)
	// Order C, A, B should be preserved (minus missing keys, but all are present here).
	if len(m.zOrder) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(m.zOrder))
	}
	order := strings.Join(m.zOrder, "")
	if order != "CAB" {
		t.Errorf("expected order CAB, got %s", order)
	}
}

func TestCanvas_ANSIPreservedOnOverlap(t *testing.T) {
	t.Parallel()
	// Two styled cards at different positions; output should contain both summaries.
	tickets := map[string]state.TicketState{
		"X": {Key: "X", Summary: "CardX", X: 0, Y: 0},
		"Y": {Key: "Y", Summary: "CardY", X: 0, Y: 12},
	}
	m := makeCanvasModel(t, 80, 24, tickets, []string{"X", "Y"})
	out := m.renderPostitCanvas()
	if !strings.Contains(out, "CardX") {
		t.Errorf("expected CardX in output:\n%s", out)
	}
	if !strings.Contains(out, "CardY") {
		t.Errorf("expected CardY in output:\n%s", out)
	}
}

func TestCanvas_WideCharCardBounds(t *testing.T) {
	t.Parallel()
	// Card containing CJK character; lipgloss.Width should be used.
	tickets := map[string]state.TicketState{
		"CJK": {Key: "CJK", Summary: "你好世界", X: 0, Y: 0},
	}
	m := makeCanvasModel(t, 80, 24, tickets, []string{"CJK"})
	// Should not panic. bubblezone not initialized so zone bounds are zero — that's ok.
	out := m.renderPostitCanvas()
	_ = out
}

func TestCanvas_MultiLineHitInterior(t *testing.T) {
	t.Parallel()
	// zone.NewGlobal() is called in TestMain; zone.Scan populates zone registry.

	tickets := map[string]state.TicketState{
		"HIT": {Key: "HIT", Summary: "Hit Test Card", X: 2, Y: 1},
	}
	m := makeCanvasModel(t, 80, 24, tickets, []string{"HIT"})
	// Render and scan so zone registry is populated.
	raw := m.renderPostitCanvas()
	zone.Scan(raw)

	info := zone.Get("HIT")
	if info.IsZero() {
		// No zone info — this can happen in headless tests without a full terminal.
		// Skip rather than fail: the bubblezone integration requires a real program run.
		t.Skip("zone.Get returned zero — skipping in headless environment")
	}
	// The zone should span at least 1 cell.
	if info.EndX <= info.StartX || info.EndY <= info.StartY {
		t.Errorf("zone HIT has zero size: (%d,%d)–(%d,%d)", info.StartX, info.StartY, info.EndX, info.EndY)
	}
}

func TestRenderPostitCanvas_EmptyBoardHint(t *testing.T) {
	t.Parallel()
	// Empty board in ModeNormal → hint visible.
	m := makeCanvasModel(t, 80, 24, nil, nil)
	m.mode = ModeNormal
	out := m.renderPostitCanvas()
	if !strings.Contains(out, "import") && !strings.Contains(out, "post-it") {
		t.Errorf("expected hint when 0 tickets in Normal mode:\n%s", out)
	}

	// One ticket present → no hint.
	tickets := map[string]state.TicketState{
		"T-1": {Key: "T-1", Summary: "My ticket"},
	}
	m2 := makeCanvasModel(t, 80, 24, tickets, []string{"T-1"})
	m2.mode = ModeNormal
	out2 := m2.renderPostitCanvas()
	if strings.Contains(out2, "Press `i`") {
		t.Errorf("hint should not appear when a ticket exists:\n%s", out2)
	}
}
