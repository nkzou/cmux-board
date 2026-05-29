// Package spatial provides pure-Go geometry helpers for the freeform 2D board.
// It has no BubbleTea, Lipgloss, or bubblezone dependencies.
package spatial

import "math"

// Positioned is a lightweight card-rect record passed in from internal/ui.
type Positioned struct {
	ID   string
	X, Y int
	W, H int
}

// Clamp clamps a card's top-left so the card stays fully inside the board.
// Returns (0, 0) if cardW > boardW or cardH > boardH (degenerate: caller responsibility).
func Clamp(x, y, cardW, cardH, boardW, boardH int) (int, int) {
	if cardW > boardW || cardH > boardH {
		return 0, 0
	}
	maxX := boardW - cardW
	maxY := boardH - cardH
	if x < 0 {
		x = 0
	} else if x > maxX {
		x = maxX
	}
	if y < 0 {
		y = 0
	} else if y > maxY {
		y = maxY
	}
	return x, y
}

// ApplyDelta returns the new card position after a cursor delta.
// Result is overflow-protected: saturates at math.MinInt / math.MaxInt.
//
//	result = (startX + (cursorNowX - cursorStartX), startY + (cursorNowY - cursorStartY))
func ApplyDelta(startX, startY, cursorStartX, cursorStartY, cursorNowX, cursorNowY int) (int, int) {
	dx := cursorNowX - cursorStartX
	dy := cursorNowY - cursorStartY
	return saturatingAdd(startX, dx), saturatingAdd(startY, dy)
}

// saturatingAdd adds a and b, saturating at math.MinInt / math.MaxInt on overflow.
func saturatingAdd(a, b int) int {
	if b > 0 && a > math.MaxInt-b {
		return math.MaxInt
	}
	if b < 0 && a < math.MinInt-b {
		return math.MinInt
	}
	return a + b
}

// HitTest finds the topmost card (last in zOrder) whose rect contains (cursorX, cursorY).
// A card's rect is [X, X+W) × [Y, Y+H) (exclusive right/bottom).
// The cards slice is used to look up rect data by ID.
// Returns ("", false) if no card contains the cursor.
func HitTest(cards []Positioned, zOrder []string, cursorX, cursorY int) (id string, found bool) {
	// Build ID→Positioned map for O(1) lookup.
	byID := make(map[string]Positioned, len(cards))
	for _, c := range cards {
		byID[c.ID] = c
	}

	// Walk zOrder from the end (top) backwards; first hit wins.
	for i := len(zOrder) - 1; i >= 0; i-- {
		k := zOrder[i]
		c, ok := byID[k]
		if !ok {
			continue
		}
		if cursorX >= c.X && cursorX < c.X+c.W &&
			cursorY >= c.Y && cursorY < c.Y+c.H {
			return k, true
		}
	}
	return "", false
}

// ZOrder moves lastInteracted to the end of the slice (highest z-order / on top).
// Preserves the relative order of all other entries.
// If lastInteracted is not in current, returns current unchanged.
// The input slice is never mutated; a new slice is returned.
func ZOrder(current []string, lastInteracted string) []string {
	found := false
	for _, k := range current {
		if k == lastInteracted {
			found = true
			break
		}
	}
	if !found {
		// Return a copy so callers cannot mutate the original.
		result := make([]string, len(current))
		copy(result, current)
		return result
	}

	result := make([]string, 0, len(current))
	for _, k := range current {
		if k != lastInteracted {
			result = append(result, k)
		}
	}
	result = append(result, lastInteracted)
	return result
}
