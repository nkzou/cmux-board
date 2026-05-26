package cmuxcli

import (
	"context"
	"testing"
)

func TestVersion_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeFakeCmux(t, dir, "#!/bin/sh\necho 'cmux 0.64.7 (87) [4d04459dd]'\n")
	prependPath(t, dir)

	v, err := Version(context.Background())
	if err != nil {
		t.Fatalf("Version error: %v", err)
	}
	if v == "" {
		t.Error("Version returned empty string")
	}
	if !containsStr(v, "0.64.7") {
		t.Errorf("Version = %q, want substring 0.64.7", v)
	}
}

func TestMeetsMinVersion(t *testing.T) {
	cases := []struct {
		name      string
		installed string
		min       string
		want      bool
	}{
		{"equal", "0.64.7", "0.64.7", true},
		{"installed greater minor", "0.65.0", "0.64.7", true},
		{"installed greater patch", "0.64.8", "0.64.7", true},
		{"installed less patch", "0.64.6", "0.64.7", false},
		{"installed less minor", "0.63.99", "0.64.7", false},
		{"installed with cmux prefix", "cmux 0.64.7 (87) [hash]", "0.64.7", true},
		// Note: pre-release suffixes like "-rc1" are not parsed; only the dot-split numeric
		// segments matter. "0.64.7-rc1" splits to {0, 64, 7-rc1} where the last segment
		// fails Atoi and becomes 0, making installed = {0, 64, 0} < min = {0, 64, 7}.
		// That is acceptable for RT-1 drift detection — pre-releases legitimately predate
		// the known-good range.
		{"installed pre-release predates known-good", "0.64.7-rc1", "0.64.7", false},
		{"installed greater major", "1.0.0", "0.64.7", true},
		{"shorter installed", "0.64", "0.64.7", false}, // missing patch defaults to 0
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := meetsMinVersion(tc.installed, tc.min)
			if got != tc.want {
				t.Errorf("meetsMinVersion(%q, %q) = %v, want %v", tc.installed, tc.min, got, tc.want)
			}
		})
	}
}
