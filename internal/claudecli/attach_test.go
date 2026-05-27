package claudecli

import (
	"encoding/json"
	"testing"
)

func TestBuildAttachCommand_Format(t *testing.T) {
	cases := []struct {
		shortID string
		want    string
	}{
		{"3174068b", "claude attach 3174068b"},
		{"cafef00d", "claude attach cafef00d"},
		{"00000000", "claude attach 00000000"},
	}
	for _, c := range cases {
		got := BuildAttachCommand(c.shortID)
		if got != c.want {
			t.Errorf("BuildAttachCommand(%q) = %q, want %q", c.shortID, got, c.want)
		}
	}
}

// layoutPane is a minimal representative of the cmux layout JSON shape that T-036 assembles.
type layoutPane struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// TestBuildAttachCommand_UsedInLayoutJSON verifies the command string round-trips through
// JSON without escaping surprises (the short_id is 8 hex chars and needs no quoting).
func TestBuildAttachCommand_UsedInLayoutJSON(t *testing.T) {
	pane := layoutPane{
		Type:    "terminal",
		Command: BuildAttachCommand("abcd1234"),
	}

	data, err := json.Marshal(pane)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded layoutPane
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	want := "claude attach abcd1234"
	if decoded.Command != want {
		t.Errorf("round-tripped Command = %q, want %q", decoded.Command, want)
	}
}
