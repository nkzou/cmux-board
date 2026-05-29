package claudecli

import (
	"strings"
	"testing"
	"time"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/tracker"
)

var testTicket = tracker.Ticket{
	ID:            "10042",
	Key:           "PROJ-42",
	Summary:       "Fix login bug",
	Status:        "In Progress",
	URL:           "https://x.atlassian.net/browse/PROJ-42",
	IssueType:     "Bug",
	AssigneeID:    "acc-123",
	AssigneeEmail: "dev@example.com",
	Labels:        []string{"auth", "backend"},
	Priority:      "High",
	UpdatedAt:     time.Date(2026, 5, 28, 12, 34, 56, 0, time.UTC),
}

var testRepo = config.RepoEntry{
	Name:          "my-service",
	Path:          "/home/user/my-service",
	DefaultBranch: "main",
}

var testData = PromptData{
	Ticket:       testTicket,
	Repo:         testRepo,
	WorktreePath: "/home/user/worktrees/my-service-PROJ-42-main-01ab23cd",
	ApproachName: "main",
}

func TestRenderPromptHappyPath(t *testing.T) {
	got, err := RenderPrompt(config.DefaultStarterPromptTemplate, testData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		"PROJ-42",
		"Fix login bug",
		"ID: 10042",
		"Issue type: Bug",
		"Status: In Progress",
		"Priority: High",
		"Assignee: dev@example.com",
		"Labels: auth backend",
		"Link: https://x.atlassian.net/browse/PROJ-42",
		"Updated: 2026-05-28 12:34:56 UTC",
		"my-service",
		"/home/user/worktrees/my-service-PROJ-42-main-01ab23cd",
		"main",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered output missing %q; got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "{{.") {
		t.Errorf("rendered output contains unexpanded template actions: %s", got)
	}
}

func TestRenderPromptDefaultTemplateHandlesMissingOptionalFields(t *testing.T) {
	data := testData
	data.Ticket = tracker.Ticket{
		Key:     "PROJ-99",
		Summary: "Sparse ticket",
	}
	got, err := RenderPrompt(config.DefaultStarterPromptTemplate, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"ID: unknown", "Issue type: unknown", "Priority: unknown", "Assignee: unknown", "Labels: none", "Link: unavailable"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered output missing %q; got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Updated:") {
		t.Errorf("zero UpdatedAt should be omitted; got:\n%s", got)
	}
}

func TestRenderPromptCustomTemplate(t *testing.T) {
	tmpl := "Ticket: {{.Ticket.Key}} Repo: {{.Repo.Name}} WT: {{.WorktreePath}}"
	got, err := RenderPrompt(tmpl, testData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Ticket: PROJ-42 Repo: my-service WT: /home/user/worktrees/my-service-PROJ-42-main-01ab23cd"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderPromptMultiRepoVars(t *testing.T) {
	tmpl := "{{.Repo.Name}}|{{.Repo.Path}}|{{.Repo.DefaultBranch}}"
	got, err := RenderPrompt(tmpl, testData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "my-service|/home/user/my-service|main"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderPromptInvalidTemplate(t *testing.T) {
	_, err := RenderPrompt("{{.Ticket.NonExistent", testData)
	if err == nil {
		t.Fatal("expected error for invalid template, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse starter_prompt_template") {
		t.Errorf("error should wrap 'failed to parse starter_prompt_template', got: %v", err)
	}
}

func TestRenderPromptExecutionError(t *testing.T) {
	// Valid parse but fails at execute because .Nonexistent doesn't exist and we use call.
	_, err := RenderPrompt("{{call .Nonexistent}}", testData)
	if err == nil {
		t.Fatal("expected error for execution failure, got nil")
	}
	if !strings.Contains(err.Error(), "failed to render starter_prompt_template") {
		t.Errorf("error should wrap 'failed to render starter_prompt_template', got: %v", err)
	}
}

func TestRenderPromptNoSideEffects(t *testing.T) {
	tmpl := "{{.Ticket.Key}}-{{.Repo.Name}}"
	first, err := RenderPrompt(tmpl, testData)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	second, err := RenderPrompt(tmpl, testData)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if first != second {
		t.Errorf("RenderPrompt is not idempotent: first=%q second=%q", first, second)
	}
}
