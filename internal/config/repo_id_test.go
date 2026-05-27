package config

import "testing"

func TestDeriveRepoID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "lowercase simple", input: "myservice", want: "myservice"},
		{name: "already-slug", input: "cmux-board", want: "cmux-board"},
		{name: "mixed case", input: "MyService", want: "myservice"},
		{name: "trailing punct", input: "MyService!", want: "myservice"},
		{name: "space sep", input: "My Repo", want: "my-repo"},
		{name: "multi space", input: "My  Repo", want: "my-repo"},
		{name: "numbers", input: "Repo 2", want: "repo-2"},
		{name: "special chars", input: "foo_bar.baz", want: "foo-bar-baz"},
		{name: "leading dashes", input: "--foo", want: "foo"},
		{name: "trailing dashes", input: "foo--", want: "foo"},
		{name: "all special", input: "---", want: ""},
		{name: "unicode run", input: "héllo", want: "h-llo"},
		{name: "full slug cmux-board", input: "cmux-board", want: "cmux-board"},
		{name: "mixed case cmux-board", input: "CmuxBoard", want: "cmuxboard"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveRepoID(tt.input)
			if got != tt.want {
				t.Errorf("DeriveRepoID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
