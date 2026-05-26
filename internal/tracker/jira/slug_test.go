package jira

import "testing"

func TestSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"To Do", "to-do"},
		{"In Progress", "in-progress"},
		{"In Progress!!", "in-progress"},
		{"Done", "done"},
		{"  --  Leading", "leading"},
		{"a--b---c", "a-b-c"},
		{"", ""},
		{"UPPER CASE 123!", "upper-case-123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Slug(tt.input)
			if got != tt.want {
				t.Errorf("Slug(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
