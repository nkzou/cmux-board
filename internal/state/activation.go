package state

// FindActivations returns the slice of activation entries bound to the (ticketID, repoID) pair.
// Returns an empty (non-nil) slice when none are found. Safe to call on a Snapshot.
// Result is a copy so callers cannot mutate the internal slice.
func FindActivations(s *State, ticketID, repoID string) []ActivationEntry {
	out := make([]ActivationEntry, 0)
	for _, act := range s.Activations[ticketID] {
		if act.RepoID == repoID {
			out = append(out, act)
		}
	}
	return out
}

// FindActivationByID returns the first entry whose activation_id matches exactly.
// Returns (nil, false) on miss. Result pointer is to a copy; mutating it does not affect state.
func FindActivationByID(s *State, activationID string) (*ActivationEntry, bool) {
	for _, entries := range s.Activations {
		for i := range entries {
			if entries[i].ActivationID == activationID {
				cp := entries[i]
				return &cp, true
			}
		}
	}
	return nil, false
}

// FindActivationByShortID returns the first entry whose act_id_short matches exactly.
// Returns (nil, false) on miss. Result pointer is to a copy; mutating it does not affect state.
func FindActivationByShortID(s *State, actIDShort string) (*ActivationEntry, bool) {
	for _, entries := range s.Activations {
		for i := range entries {
			if entries[i].ActIDShort == actIDShort {
				cp := entries[i]
				return &cp, true
			}
		}
	}
	return nil, false
}

// ActivationCount returns the number of non-removed activations for ticketID,
// across all repos. Used by the UI to render the worktree-exists badge on
// each ticket card.
func ActivationCount(s *State, ticketID string) int {
	n := 0
	for _, act := range s.Activations[ticketID] {
		if act.RemovedAt == nil {
			n++
		}
	}
	return n
}

// AllIncompleteActivations returns all entries across all tickets and repos where complete == false.
// Used by startup reconciliation (T-062a).
func AllIncompleteActivations(s *State) []ActivationEntry {
	out := make([]ActivationEntry, 0)
	for _, entries := range s.Activations {
		for _, act := range entries {
			if !act.Complete {
				out = append(out, act)
			}
		}
	}
	return out
}
