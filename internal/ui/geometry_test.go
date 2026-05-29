package ui

import (
	"math"
	"testing"
)

func TestClamp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                               string
		x, y, cardW, cardH, boardW, boardH int
		wantX, wantY                       int
	}{
		{"inside", 5, 5, 3, 2, 20, 20, 5, 5},
		{"clamp left", -3, 5, 3, 2, 20, 20, 0, 5},
		{"clamp right", 100, 5, 3, 2, 20, 20, 17, 5},
		{"clamp top", 5, -1, 3, 2, 20, 20, 5, 0},
		{"clamp bottom", 5, 100, 3, 2, 20, 20, 5, 18},
		{"clamp left+top", -5, -5, 3, 2, 20, 20, 0, 0},
		{"clamp right+bottom", 100, 100, 3, 2, 20, 20, 17, 18},
		{"exactly at right edge", 17, 5, 3, 2, 20, 20, 17, 5},
		{"exactly at bottom edge", 5, 18, 3, 2, 20, 20, 5, 18},
		{"card fills board exactly", 0, 0, 20, 20, 20, 20, 0, 0},
		{"degenerate: cardW > boardW", 5, 5, 25, 2, 20, 20, 0, 0},
		{"degenerate: cardH > boardH", 5, 5, 3, 25, 20, 20, 0, 0},
		{"at origin inside", 0, 0, 3, 2, 20, 20, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			x, y := Clamp(tt.x, tt.y, tt.cardW, tt.cardH, tt.boardW, tt.boardH)
			if x != tt.wantX || y != tt.wantY {
				t.Errorf("got (%d,%d), want (%d,%d)", x, y, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestApplyDelta(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                                                              string
		startX, startY, cursorStartX, cursorStartY, cursorNowX, cursorNowY int
		wantX, wantY                                                      int
	}{
		{"standard positive delta", 10, 10, 5, 5, 8, 9, 13, 14},
		{"zero delta", 10, 10, 5, 5, 5, 5, 10, 10},
		{"negative delta", 10, 10, 8, 9, 5, 5, 7, 6},
		{"large positive delta", 100, 100, 0, 0, 1000, 1000, 1100, 1100},
		{"start at origin", 0, 0, 0, 0, 5, 3, 5, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			x, y := ApplyDelta(tt.startX, tt.startY, tt.cursorStartX, tt.cursorStartY, tt.cursorNowX, tt.cursorNowY)
			if x != tt.wantX || y != tt.wantY {
				t.Errorf("got (%d,%d), want (%d,%d)", x, y, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestApplyDelta_OverflowProtected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                                                              string
		startX, startY, cursorStartX, cursorStartY, cursorNowX, cursorNowY int
		wantX, wantY                                                      int
	}{
		{
			name:         "x overflow positive",
			startX:       math.MaxInt - 5,
			startY:       0,
			cursorStartX: 0,
			cursorStartY: 0,
			cursorNowX:   100,
			cursorNowY:   0,
			wantX:        math.MaxInt,
			wantY:        0,
		},
		{
			name:         "x overflow negative",
			startX:       math.MinInt + 5,
			startY:       0,
			cursorStartX: 100,
			cursorStartY: 0,
			cursorNowX:   0,
			cursorNowY:   0,
			wantX:        math.MinInt,
			wantY:        0,
		},
		{
			name:         "y overflow positive",
			startX:       0,
			startY:       math.MaxInt - 3,
			cursorStartX: 0,
			cursorStartY: 0,
			cursorNowX:   0,
			cursorNowY:   10,
			wantX:        0,
			wantY:        math.MaxInt,
		},
		{
			name:         "y overflow negative",
			startX:       0,
			startY:       math.MinInt + 2,
			cursorStartX: 0,
			cursorStartY: 5,
			cursorNowX:   0,
			cursorNowY:   0,
			wantX:        0,
			wantY:        math.MinInt,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			x, y := ApplyDelta(tt.startX, tt.startY, tt.cursorStartX, tt.cursorStartY, tt.cursorNowX, tt.cursorNowY)
			if x != tt.wantX || y != tt.wantY {
				t.Errorf("got (%d,%d), want (%d,%d)", x, y, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestHitTest_NoOverlap(t *testing.T) {
	t.Parallel()
	cards := []Positioned{
		{ID: "A", X: 0, Y: 0, W: 5, H: 3},
		{ID: "B", X: 10, Y: 10, W: 5, H: 3},
	}
	zOrder := []string{"A", "B"}
	id, found := HitTest(cards, zOrder, 20, 20)
	if found {
		t.Errorf("expected no hit, got %q", id)
	}
}

func TestHitTest_TwoOverlapping_LastOnTopWins(t *testing.T) {
	t.Parallel()
	cards := []Positioned{
		{ID: "A", X: 0, Y: 0, W: 10, H: 5},
		{ID: "B", X: 2, Y: 1, W: 10, H: 5}, // overlaps with A
	}
	zOrder := []string{"A", "B"} // B is on top (last in zOrder)
	id, found := HitTest(cards, zOrder, 3, 2)
	if !found || id != "B" {
		t.Errorf("expected hit on B, got found=%v id=%q", found, id)
	}
}

func TestHitTest_OverlapReversedZOrder_OtherCardWins(t *testing.T) {
	t.Parallel()
	cards := []Positioned{
		{ID: "A", X: 0, Y: 0, W: 10, H: 5},
		{ID: "B", X: 2, Y: 1, W: 10, H: 5}, // overlaps with A
	}
	zOrder := []string{"B", "A"} // A is on top now
	id, found := HitTest(cards, zOrder, 3, 2)
	if !found || id != "A" {
		t.Errorf("expected hit on A, got found=%v id=%q", found, id)
	}
}

func TestHitTest_SingleCard(t *testing.T) {
	t.Parallel()
	cards := []Positioned{
		{ID: "A", X: 5, Y: 3, W: 8, H: 4},
	}
	zOrder := []string{"A"}

	// Inside: top-left corner
	id, found := HitTest(cards, zOrder, 5, 3)
	if !found || id != "A" {
		t.Errorf("top-left: expected A, got found=%v id=%q", found, id)
	}

	// Inside: bottom-right corner (exclusive bounds: X+W-1, Y+H-1)
	id, found = HitTest(cards, zOrder, 12, 6)
	if !found || id != "A" {
		t.Errorf("bottom-right: expected A, got found=%v id=%q", found, id)
	}

	// Outside: just past right edge
	_, found = HitTest(cards, zOrder, 13, 3)
	if found {
		t.Errorf("just past right edge: expected no hit")
	}

	// Outside: just past bottom edge
	_, found = HitTest(cards, zOrder, 5, 7)
	if found {
		t.Errorf("just past bottom edge: expected no hit")
	}
}

func TestZOrder_CyclesLastInteractedToTop_PreservesRest(t *testing.T) {
	t.Parallel()
	current := []string{"A", "B", "C", "D"}
	result := ZOrder(current, "B")
	want := []string{"A", "C", "D", "B"}
	if len(result) != len(want) {
		t.Fatalf("len: got %d, want %d", len(result), len(want))
	}
	for i := range want {
		if result[i] != want[i] {
			t.Errorf("[%d]: got %q, want %q", i, result[i], want[i])
		}
	}
}

func TestZOrder_MissingKey_Unchanged(t *testing.T) {
	t.Parallel()
	current := []string{"A", "B", "C"}
	result := ZOrder(current, "X")
	if len(result) != len(current) {
		t.Fatalf("len: got %d, want %d", len(result), len(current))
	}
	for i := range current {
		if result[i] != current[i] {
			t.Errorf("[%d]: got %q, want %q", i, result[i], current[i])
		}
	}
}

func TestZOrder_AlreadyOnTop_Unchanged(t *testing.T) {
	t.Parallel()
	current := []string{"A", "B", "C"}
	result := ZOrder(current, "C")
	// C is already last; result should be ["A", "B", "C"]
	want := []string{"A", "B", "C"}
	for i := range want {
		if result[i] != want[i] {
			t.Errorf("[%d]: got %q, want %q", i, result[i], want[i])
		}
	}
}

func TestZOrder_EmptySlice(t *testing.T) {
	t.Parallel()
	result := ZOrder(nil, "A")
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestHitTest_EmptyCards(t *testing.T) {
	t.Parallel()
	_, found := HitTest(nil, nil, 5, 5)
	if found {
		t.Error("expected no hit on empty cards")
	}
}
