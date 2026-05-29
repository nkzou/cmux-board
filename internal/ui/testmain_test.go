package ui

import (
	"os"
	"testing"

	zone "github.com/lrstanley/bubblezone"
)

// TestMain initializes the bubblezone global manager before any test in the ui
// package runs. zone.NewGlobal() writes the package-level DefaultManager pointer;
// initializing it once here avoids data races between parallel tests that either
// call zone.Mark or read DefaultManager.
func TestMain(m *testing.M) {
	zone.NewGlobal()
	os.Exit(m.Run())
}
