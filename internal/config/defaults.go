package config

// DefaultStarterPromptTemplate is the default value for
// config.json "claude.starter_prompt".
// Available template variables: {{.Ticket.*}}, {{.Repo.*}},
// {{.WorktreePath}}, {{.ApproachName}}.
const DefaultStarterPromptTemplate = `You are starting work on ticket {{.Ticket.Key}}: {{.Ticket.Summary}}.

Jira context:
- ID: {{if .Ticket.ID}}{{.Ticket.ID}}{{else}}unknown{{end}}
- Issue type: {{if .Ticket.IssueType}}{{.Ticket.IssueType}}{{else}}unknown{{end}}
- Status: {{if .Ticket.Status}}{{.Ticket.Status}}{{else}}unknown{{end}}
- Priority: {{if .Ticket.Priority}}{{.Ticket.Priority}}{{else}}unknown{{end}}
- Assignee: {{if .Ticket.AssigneeEmail}}{{.Ticket.AssigneeEmail}}{{else if .Ticket.AssigneeID}}{{.Ticket.AssigneeID}}{{else}}unknown{{end}}
- Labels:{{if .Ticket.Labels}}{{range .Ticket.Labels}} {{.}}{{end}}{{else}} none{{end}}
- Link: {{if .Ticket.URL}}{{.Ticket.URL}}{{else}}unavailable{{end}}{{if not .Ticket.UpdatedAt.IsZero}}
- Updated: {{.Ticket.UpdatedAt.Format "2006-01-02 15:04:05 MST"}}{{end}}

Workspace context:
- Repo: {{.Repo.Name}} ({{.Repo.Path}})
- Worktree: {{.WorktreePath}}
- Approach: {{.ApproachName}}

Read the worktree and Jira context to orient yourself, then ask me what to do first.`
