package ui

import (
	"strings"
	"testing"
)

// TestHelpText_MentionsNewKeybinds verifies that HelpText references all new
// post-it board keybinds and does not mention obsolete column navigation.
func TestHelpText_MentionsNewKeybinds(t *testing.T) {
	t.Parallel()
	help := HelpText()

	required := []string{
		KeyImportJira,   // "i"
		KeyCreateLocal,  // "c"
		KeyRemoveTicket, // "x"
		KeyCycleStatus,  // "s"
		KeyZCycleNext,   // "tab"
	}
	for _, want := range required {
		if !strings.Contains(help, want) {
			t.Errorf("help text missing keybind %q", want)
		}
	}

	banned := []string{"left column", "right column"}
	for _, b := range banned {
		if strings.Contains(help, b) {
			t.Errorf("help text still mentions obsolete string %q", b)
		}
	}
}
