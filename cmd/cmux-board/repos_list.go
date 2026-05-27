package main

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/kevin-zou/cmux-board/internal/config"
	"github.com/kevin-zou/cmux-board/internal/state"
)

func newReposListCmd() *cobra.Command {
	var flagJSON bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered repositories",
		Long: `List all repositories registered in config.json.

Default output is a tab-aligned table:
  ID  NAME  PATH  DEFAULT_BRANCH  REFS

With --json, outputs a JSON array of objects with the same fields plus
  "ticket_refs"     (number of tickets with this repo in assigned_repo_ids)
  "activation_refs" (number of activation entries with this repo_id)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := config.DefaultConfigPath()
			if err != nil {
				return fmt.Errorf("failed to resolve config path: %w", err)
			}
			statePath, err := config.DefaultStatePath()
			if err != nil {
				return fmt.Errorf("failed to resolve state path: %w", err)
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// State is optional; missing or corrupt state.json is non-fatal for list.
			var st *state.State
			store, stErr := state.Open(statePath)
			if stErr == nil {
				st, _ = store.Snapshot()
			}
			// stErr != nil: repos list degrades gracefully to 0 ref counts.

			rows := config.ListRepos(&cfg, st)
			w := cmd.OutOrStdout()

			if flagJSON {
				return printReposJSON(w, rows)
			}
			return printReposTable(w, rows)
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "Output as JSON array")
	return cmd
}

// printReposTable outputs a tab-aligned table to w.
func printReposTable(w io.Writer, rows []config.RepoListEntry) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tPATH\tDEFAULT_BRANCH\tREFS")
	fmt.Fprintln(tw, "--\t----\t----\t--------------\t----")
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d tickets / %d activations\n",
			row.ID, row.Name, row.Path, row.DefaultBranch,
			row.TicketRefs, row.ActivationRefs)
	}
	return tw.Flush()
}

// repoJSONRow is the schema for --json output.
type repoJSONRow struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Path           string `json:"path"`
	DefaultBranch  string `json:"default_branch"`
	TicketRefs     int    `json:"ticket_refs"`
	ActivationRefs int    `json:"activation_refs"`
}

// printReposJSON outputs a JSON array to w.
func printReposJSON(w io.Writer, rows []config.RepoListEntry) error {
	out := make([]repoJSONRow, len(rows))
	for i, row := range rows {
		out[i] = repoJSONRow{
			ID:             row.ID,
			Name:           row.Name,
			Path:           row.Path,
			DefaultBranch:  row.DefaultBranch,
			TicketRefs:     row.TicketRefs,
			ActivationRefs: row.ActivationRefs,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
