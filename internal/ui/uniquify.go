package ui

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrUniquifyExhausted is returned when all 99 suffix slots are occupied.
var ErrUniquifyExhausted = errors.New("uniquify: all path suffixes 2–99 exhausted")

// Uniquify checks whether path exists on disk. If it does not, it returns
// (path, "", nil). If it does, it tries path+"-2", path+"-3", … path+"-99"
// and returns the first absent path with its suffix string (e.g., "-2").
// If all 99 suffixes are occupied, it returns ("", "", ErrUniquifyExhausted).
//
// The path argument should NOT have a trailing slash; callers are responsible
// for any trailing slash convention. The returned suffix is always "" or "-N"
// for N ∈ [2, 99] — it never contains a trailing slash.
func Uniquify(path string) (resultPath string, suffix string, err error) {
	// Strip trailing slash for stat/suffix operations; re-add at the end.
	hasTrailingSlash := strings.HasSuffix(path, "/")
	base := strings.TrimSuffix(path, "/")

	if _, statErr := os.Stat(base); os.IsNotExist(statErr) {
		return path, "", nil
	}
	for n := 2; n <= 99; n++ {
		sfx := fmt.Sprintf("-%d", n)
		candidate := base + sfx
		if _, statErr := os.Stat(candidate); os.IsNotExist(statErr) {
			if hasTrailingSlash {
				candidate += "/"
			}
			return candidate, sfx, nil
		}
	}
	return "", "", ErrUniquifyExhausted
}
