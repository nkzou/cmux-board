package config

// DefaultStarterPromptTemplate is the default value for
// config.json "claude.starter_prompt_template".
// Available template variables: {{.Ticket.*}}, {{.Repo.*}},
// {{.WorktreePath}}, {{.ApproachName}}.
const DefaultStarterPromptTemplate = `You are starting work on ticket {{.Ticket.Key}}: {{.Ticket.Summary}}.
Status: {{.Ticket.Status}}. Link: {{.Ticket.URL}}.
Repo: {{.Repo.Name}} ({{.Repo.Path}}). Worktree: {{.WorktreePath}}. Approach: {{.ApproachName}}.
Read the worktree to orient yourself, then ask me what to do first.`
