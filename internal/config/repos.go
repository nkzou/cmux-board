package config

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/nkzou/cmux-board/internal/state"
)

// RepoArgs is the input to AddRepo.
type RepoArgs struct {
	ID            string // kebab-slug; caller derives via DeriveRepoID if empty
	Name          string // display name
	Path          string // absolute path to the git clone (already validated by T-047)
	DefaultBranch string // branch ref (already validated by T-047)
}

// ErrDuplicateRepoID is returned by AddRepo when the id is already in cfg.Repos.
var ErrDuplicateRepoID = errors.New("repo_id already registered")

// ErrRepoReferenced is returned by RemoveRepo when the id is still referenced by
// a ticket's assigned_repo_ids or an activation's repo_id.
var ErrRepoReferenced = errors.New("repo is still referenced")

// AddRepo inserts a new entry into cfg.Repos.
// Preconditions (caller must have verified before calling):
//   - args.Path is a valid git repo (ValidateRepoPath).
//   - args.DefaultBranch resolves (ValidateDefaultBranch).
//   - args.ID is non-empty (derived or explicitly supplied).
//
// Returns ErrDuplicateRepoID if args.ID already exists in cfg.Repos.
// Does NOT write to disk; caller is responsible for atomic config write.
func AddRepo(cfg *Config, args RepoArgs) error {
	if cfg.Repos == nil {
		cfg.Repos = make(map[string]RepoEntry)
	}
	if _, exists := cfg.Repos[args.ID]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateRepoID, args.ID)
	}
	cfg.Repos[args.ID] = RepoEntry{
		Name:          args.Name,
		Path:          args.Path,
		DefaultBranch: args.DefaultBranch,
	}
	return nil
}

// RepoListEntry is the display row returned by ListRepos.
type RepoListEntry struct {
	ID             string
	Name           string
	Path           string
	DefaultBranch  string
	TicketRefs     int // number of tickets whose assigned_repo_ids includes this ID
	ActivationRefs int // number of activation entries whose repo_id matches this ID
}

// ListRepos returns a slice of RepoListEntry derived from cfg.Repos and st.
// st may be nil (e.g., before state.json exists); in that case ref counts are 0.
// Order: sorted by ID for deterministic output.
func ListRepos(cfg *Config, st *state.State) []RepoListEntry {
	result := make([]RepoListEntry, 0, len(cfg.Repos))
	for id, entry := range cfg.Repos {
		row := RepoListEntry{
			ID:            id,
			Name:          entry.Name,
			Path:          entry.Path,
			DefaultBranch: entry.DefaultBranch,
		}
		if st != nil {
			row.TicketRefs, row.ActivationRefs = countRefs(st, id)
		}
		result = append(result, row)
	}
	sortRepoListEntries(result)
	return result
}

// RemoveRepo deletes the entry with the given id from cfg.Repos.
// It first scans st.Tickets (including removed_at-marked tickets, per Codex Finding 9 /
// E-NEW5) and st.Activations for any reference to id. If any reference exists, it
// returns ErrRepoReferenced with the offending ticket keys listed in the error message
// so the user can clear them via the picker assignment editor first.
// Does NOT write to disk; caller is responsible for atomic config write.
func RemoveRepo(cfg *Config, st *state.State, id string) error {
	if _, exists := cfg.Repos[id]; !exists {
		return fmt.Errorf("repo_id %q not found in config", id)
	}
	// Collect all referencing ticket keys (including removed_at-marked tickets).
	refSet := make(map[string]struct{})
	for key, ticket := range st.Tickets {
		for _, rid := range ticket.AssignedRepoIDs {
			if rid == id {
				refSet[key] = struct{}{}
				break
			}
		}
	}
	for ticketKey, activations := range st.Activations {
		for _, act := range activations {
			if act.RepoID == id {
				refSet[ticketKey] = struct{}{}
				break
			}
		}
	}
	if len(refSet) > 0 {
		keys := make([]string, 0, len(refSet))
		for k := range refSet {
			keys = append(keys, k)
		}
		sort.Strings(keys) // deterministic order for testing
		return fmt.Errorf("%w: %q is still referenced by ticket(s): %s — clear assignments via picker `a` first",
			ErrRepoReferenced, id, strings.Join(keys, ", "))
	}
	delete(cfg.Repos, id)
	return nil
}

// countRefs counts the number of tickets and activation entries referencing the given repo id.
func countRefs(st *state.State, id string) (ticketRefs, activationRefs int) {
	for _, ticket := range st.Tickets {
		for _, rid := range ticket.AssignedRepoIDs {
			if rid == id {
				ticketRefs++
				break
			}
		}
	}
	for _, activations := range st.Activations {
		for _, act := range activations {
			if act.RepoID == id {
				activationRefs++
			}
		}
	}
	return
}

// sortRepoListEntries sorts entries by ID ascending.
func sortRepoListEntries(entries []RepoListEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})
}
