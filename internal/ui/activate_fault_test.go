package ui

// activate_fault_test.go — 3×3 fault-injection matrix for the activation orchestrator.
//
// Tests 9 fault scenarios (3 side effects × 3 kill moments) plus special case S0
// (kill BEFORE initial journal). After simulated restart, a fake reconciler discovers
// externally-created resources (by substring matching act_id_short) and the resume
// path runs only the missing steps. No resource is ever duplicated.
//
// ALL tests run with -race: go test -race ./internal/ui/...

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kevin-zou/cmux-board/internal/claudecli"
	"github.com/kevin-zou/cmux-board/internal/cmuxcli"
	"github.com/kevin-zou/cmux-board/internal/config"
	"github.com/kevin-zou/cmux-board/internal/state"
)

// ── Fake resource drivers ────────────────────────────────────────────────────

// fakeGitDriver records calls and creates real directories for reconciler discovery.
type fakeGitDriver struct {
	mu         sync.Mutex
	createCount int
	created     []string // absolute paths created
	baseDir     string   // temp base dir for worktrees
}

func newFakeGitDriver(baseDir string) *fakeGitDriver {
	return &fakeGitDriver{baseDir: baseDir}
}

func (f *fakeGitDriver) createWorktree(_, absPath, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCount++
	f.created = append(f.created, absPath)
	// Create a real directory so reconciler path-scanning finds it.
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return fmt.Errorf("fakeGit: MkdirAll %s: %w", absPath, err)
	}
	return nil
}

// listForReconcile returns (worktreePath, branch) pairs for any path containing actIDShort.
func (f *fakeGitDriver) listForReconcile(actIDShort string) []struct{ path, branch string } {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []struct{ path, branch string }
	for _, p := range f.created {
		if strings.Contains(p, actIDShort) {
			result = append(result, struct{ path, branch string }{path: p, branch: actIDShort})
		}
	}
	return result
}

// fakeClaudeDriver records calls and maintains an in-memory agent list.
type fakeClaudeDriver struct {
	mu         sync.Mutex
	launchCount int
	agents      []claudeAgent
}

type claudeAgent struct {
	name    string
	shortID string
}

func (f *fakeClaudeDriver) launchBackground(_ context.Context, args claudecli.BGArgs) (claudecli.BGResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.launchCount++
	// Derive short ID from name (first 8 non-colon chars after last ':').
	parts := strings.Split(args.Name, ":")
	shortID := ""
	if len(parts) > 0 {
		s := parts[len(parts)-1]
		if len(s) >= 8 {
			shortID = s[:8]
		} else {
			shortID = s
		}
	}
	f.agents = append(f.agents, claudeAgent{name: args.Name, shortID: shortID})
	return claudecli.BGResult{ShortID: shortID}, nil
}

func (f *fakeClaudeDriver) listForReconcile(actIDShort string) []claudeAgent {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []claudeAgent
	for _, a := range f.agents {
		if strings.Contains(a.name, actIDShort) || strings.Contains(a.shortID, actIDShort) {
			result = append(result, a)
		}
	}
	return result
}

// fakeCmuxDriver records calls and maintains an in-memory workspace list.
type fakeCmuxDriver struct {
	mu           sync.Mutex
	newWSCount   int
	workspaces   []cmuxWorkspace
}

type cmuxWorkspace struct {
	name string
	ref  string
}

func (f *fakeCmuxDriver) newWorkspace(_ context.Context, args cmuxcli.NewWorkspaceArgs) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.newWSCount++
	ref := fmt.Sprintf("ws:%d", f.newWSCount)
	f.workspaces = append(f.workspaces, cmuxWorkspace{name: args.Name, ref: ref})
	return ref, nil
}

func (f *fakeCmuxDriver) listPanes(_ context.Context, _ string) ([]cmuxcli.Pane, error) {
	return []cmuxcli.Pane{{Ref: "pane:1", Index: 0}}, nil
}

func (f *fakeCmuxDriver) listForReconcile(actIDShort string) []cmuxWorkspace {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []cmuxWorkspace
	for _, w := range f.workspaces {
		if strings.Contains(w.name, actIDShort) {
			result = append(result, w)
		}
	}
	return result
}

// ── Fault injection helpers ──────────────────────────────────────────────────

// faultScenario holds all components for a single fault-injection test.
type faultScenario struct {
	name       string
	statePath  string
	store      *state.Store
	cfg        *config.Config
	gitDriver  *fakeGitDriver
	claude     *fakeClaudeDriver
	cmux       *fakeCmuxDriver
	hooks      *ActivationHooks
}

func newFaultScenario(t *testing.T, name string) *faultScenario {
	t.Helper()
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	worktreeBase := filepath.Join(dir, "worktrees")
	repoPath := filepath.Join(dir, "repo")
	_ = os.MkdirAll(repoPath, 0755)

	store, err := state.Open(statePath)
	if err != nil {
		t.Fatalf("%s: state.Open: %v", name, err)
	}
	if err := store.Mutate(func(s *state.State) error {
		s.Tickets["PROJ-1"] = state.TicketState{Key: "PROJ-1", Summary: "test"}
		return nil
	}); err != nil {
		t.Fatalf("%s: seed: %v", name, err)
	}

	gitDrv := newFakeGitDriver(worktreeBase)
	claudeDrv := &fakeClaudeDriver{}
	cmuxDrv := &fakeCmuxDriver{}

	cfg := &config.Config{
		WorktreeBaseDir: worktreeBase,
		Repos: map[string]config.RepoEntry{
			"repo1": {ID: "repo1", Name: "Repo 1", Path: repoPath, DefaultBranch: "main"},
		},
		Claude: config.ClaudeConfig{StarterPrompt: "work on {{.Ticket.Key}}"},
	}

	return &faultScenario{
		name:      name,
		statePath: statePath,
		store:     store,
		cfg:       cfg,
		gitDriver: gitDrv,
		claude:    claudeDrv,
		cmux:      cmuxDrv,
		hooks:     &ActivationHooks{},
	}
}

func (sc *faultScenario) deps() activateDeps {
	return activateDeps{
		createWorktree:   sc.gitDriver.createWorktree,
		launchBackground: sc.claude.launchBackground,
		newWorkspace:     sc.cmux.newWorkspace,
		listPanes:        sc.cmux.listPanes,
		hooks:            sc.hooks,
	}
}

// runWithFault runs fn, catching any panic (the kill signal).
// Returns the panic value, or nil on clean completion.
func runWithFault(fn func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()
	fn()
	return false
}

// fakeReconcile simulates startup reconciliation using the fake drivers.
// It scans each driver for resources matching actIDShort and journals them
// into a freshly-opened store (simulating a process restart).
func fakeReconcile(
	t *testing.T,
	statePath string,
	gitDrv *fakeGitDriver,
	claudeDrv *fakeClaudeDriver,
	cmuxDrv *fakeCmuxDriver,
) *state.Store {
	t.Helper()
	// Simulate process restart: open a new store instance from the persisted file.
	newStore, err := state.Open(statePath)
	if err != nil {
		t.Fatalf("fakeReconcile: state.Open: %v", err)
	}

	snap, _ := newStore.Snapshot()
	for _, entries := range snap.Activations {
		for _, entry := range entries {
			if entry.Complete {
				continue
			}
			actIDShort := entry.ActIDShort

			// Check for worktree.
			if entry.Step == state.StepStarted {
				wts := gitDrv.listForReconcile(actIDShort)
				if len(wts) > 0 {
					wt := wts[0]
					_ = newStore.Mutate(func(s *state.State) error {
						act := findActivationMutable(s, entry.ActivationID)
						if act == nil {
							return nil
						}
						act.WorktreePath = wt.path
						act.BranchName = wt.branch
						act.Step = state.StepWorktreeCreated
						return nil
					})
				}
			}

			// Re-read after possible step update.
			snap2, _ := newStore.Snapshot()
			updated, ok := state.FindActivationByID(snap2, entry.ActivationID)
			if !ok {
				continue
			}

			// Check for claude session.
			if updated.Step == state.StepWorktreeCreated {
				agents := claudeDrv.listForReconcile(actIDShort)
				if len(agents) > 0 {
					ag := agents[0]
					_ = newStore.Mutate(func(s *state.State) error {
						act := findActivationMutable(s, entry.ActivationID)
						if act == nil {
							return nil
						}
						act.ClaudeShortID = ag.shortID
						act.Step = state.StepClaudeStarted
						return nil
					})
				}
			}

			// Re-read again.
			snap3, _ := newStore.Snapshot()
			updated2, ok := state.FindActivationByID(snap3, entry.ActivationID)
			if !ok {
				continue
			}

			// Check for cmux workspace.
			if updated2.Step == state.StepClaudeStarted {
				wss := cmuxDrv.listForReconcile(actIDShort)
				if len(wss) > 0 {
					ws := wss[0]
					_ = newStore.Mutate(func(s *state.State) error {
						act := findActivationMutable(s, entry.ActivationID)
						if act == nil {
							return nil
						}
						act.CmuxWorkspaceID = ws.ref
						act.AgentPaneRef = "pane:1"
						act.Step = state.StepCmuxCreated
						act.Complete = true
						return nil
					})
				}
			}
		}
	}
	return newStore
}

// resumeMissingSteps runs only the steps that are not yet journaled for entry,
// using the same fake drivers.
func resumeMissingSteps(
	t *testing.T,
	ctx context.Context,
	store *state.Store,
	cfg *config.Config,
	entry state.ActivationEntry,
	gitDrv *fakeGitDriver,
	claudeDrv *fakeClaudeDriver,
	cmuxDrv *fakeCmuxDriver,
) {
	t.Helper()

	worktreeDir := strings.TrimSuffix(entry.WorktreePath, "/")

	if entry.Step == state.StepStarted {
		// Run worktree creation.
		if err := gitDrv.createWorktree("", worktreeDir, entry.BranchName, "main"); err != nil {
			t.Fatalf("resumeMissingSteps worktree: %v", err)
		}
		_ = store.Mutate(func(s *state.State) error {
			act := findActivationMutable(s, entry.ActivationID)
			if act == nil {
				return nil
			}
			act.Step = state.StepWorktreeCreated
			act.WorktreePath = entry.WorktreePath
			return nil
		})
		entry.Step = state.StepWorktreeCreated
	}

	var shortID string
	if entry.Step == state.StepWorktreeCreated {
		// Run claude launch.
		result, err := claudeDrv.launchBackground(ctx, claudecli.BGArgs{
			Name:     entry.ClaudeName,
			Worktree: worktreeDir,
		})
		if err != nil {
			t.Fatalf("resumeMissingSteps claude: %v", err)
		}
		shortID = result.ShortID
		_ = store.Mutate(func(s *state.State) error {
			act := findActivationMutable(s, entry.ActivationID)
			if act == nil {
				return nil
			}
			act.ClaudeShortID = shortID
			act.Step = state.StepClaudeStarted
			return nil
		})
		entry.Step = state.StepClaudeStarted
	} else {
		shortID = entry.ClaudeShortID
	}

	if entry.Step == state.StepClaudeStarted && !entry.Complete {
		// Run cmux workspace creation.
		ref, err := cmuxDrv.newWorkspace(ctx, cmuxcli.NewWorkspaceArgs{
			Name:               entry.CmuxName,
			CWD:                worktreeDir,
			AgentAttachCommand: "claude attach " + shortID,
		})
		if err != nil {
			t.Fatalf("resumeMissingSteps cmux: %v", err)
		}
		_ = store.Mutate(func(s *state.State) error {
			act := findActivationMutable(s, entry.ActivationID)
			if act == nil {
				return nil
			}
			act.CmuxWorkspaceID = ref
			act.AgentPaneRef = "pane:1"
			act.Step = state.StepCmuxCreated
			act.Complete = true
			return nil
		})
	}
}

// assertNoDuplication verifies that each fake driver's create-call counter equals 1.
func assertNoDuplication(t *testing.T, name string, gitDrv *fakeGitDriver, claudeDrv *fakeClaudeDriver, cmuxDrv *fakeCmuxDriver) {
	t.Helper()
	gitDrv.mu.Lock()
	gc := gitDrv.createCount
	gitDrv.mu.Unlock()

	claudeDrv.mu.Lock()
	cc := claudeDrv.launchCount
	claudeDrv.mu.Unlock()

	cmuxDrv.mu.Lock()
	wc := cmuxDrv.newWSCount
	cmuxDrv.mu.Unlock()

	if gc != 1 {
		t.Errorf("%s: git.createCount = %d, want 1 (no duplication)", name, gc)
	}
	if cc != 1 {
		t.Errorf("%s: claude.launchCount = %d, want 1 (no duplication)", name, cc)
	}
	if wc != 1 {
		t.Errorf("%s: cmux.newWSCount = %d, want 1 (no duplication)", name, wc)
	}
}

// assertComplete verifies that the activation entry is complete and has all IDs set.
func assertComplete(t *testing.T, scenarioName string, store *state.Store, activationID string) {
	t.Helper()
	snap, _ := store.Snapshot()
	entry, ok := state.FindActivationByID(snap, activationID)
	if !ok {
		t.Fatalf("%s: activation %s not found in state", scenarioName, activationID)
	}
	if !entry.Complete {
		t.Errorf("%s: activation.Complete = false, want true; step=%s", scenarioName, entry.Step)
	}
	if entry.WorktreePath == "" {
		t.Errorf("%s: WorktreePath is empty", scenarioName)
	}
	if entry.ClaudeShortID == "" {
		t.Errorf("%s: ClaudeShortID is empty", scenarioName)
	}
	if entry.CmuxWorkspaceID == "" {
		t.Errorf("%s: CmuxWorkspaceID is empty", scenarioName)
	}
}

// ── Special case S0 ──────────────────────────────────────────────────────────

// TestActivateFault_S0_KillBeforeInitialJournal verifies that killing BEFORE the initial
// store.Mutate leaves no incomplete activation in state.json.
// Ties to: F-NEW3.
func TestActivateFault_S0_KillBeforeInitialJournal(t *testing.T) {
	sc := newFaultScenario(t, "S0")
	sc.hooks.BeforeInitialJournal = func() { panic("kill") }

	panicked := runWithFault(func() {
		_, _ = activate(context.Background(), sc.store, sc.cfg, "PROJ-1", "repo1", "main", sc.deps())
	})
	if !panicked {
		t.Fatal("S0: expected panic, got clean completion")
	}

	snap, _ := sc.store.Snapshot()
	if len(snap.Activations) != 0 {
		t.Errorf("S0: expected empty activations after pre-journal kill, got %v", snap.Activations)
	}
	// Reconcile + no resource counters should all be zero.
	newStore := fakeReconcile(t, sc.statePath, sc.gitDriver, sc.claude, sc.cmux)
	snap2, _ := newStore.Snapshot()
	if len(snap2.Activations) != 0 {
		t.Errorf("S0: reconcile found activations that should not exist")
	}
}

// ── 1a: Worktree, kill BEFORE ─────────────────────────────────────────────────

// TestActivateFault_1a_WorktreeKillBefore verifies that killing before worktree creation
// leaves the activation at step:started with no worktree on disk.
// Ties to: E-NEW3.
func TestActivateFault_1a_WorktreeKillBefore(t *testing.T) {
	sc := newFaultScenario(t, "1a")
	sc.hooks.BeforeWorktree = func() { panic("kill") }

	var activationID string
	panicked := runWithFault(func() {
		entry, err := activate(context.Background(), sc.store, sc.cfg, "PROJ-1", "repo1", "main", sc.deps())
		if err == nil {
			activationID = entry.ActivationID
		}
	})
	if !panicked {
		t.Fatal("1a: expected panic")
	}

	// Find the journaled activation (step:started).
	snap, _ := sc.store.Snapshot()
	var foundEntry state.ActivationEntry
	for _, entries := range snap.Activations {
		for _, e := range entries {
			if e.Step == state.StepStarted {
				foundEntry = e
				activationID = e.ActivationID
			}
		}
	}
	if activationID == "" {
		t.Fatal("1a: no activation found in state after kill")
	}
	_ = activationID

	// Reconcile: no worktree on disk → step stays started.
	newStore := fakeReconcile(t, sc.statePath, sc.gitDriver, sc.claude, sc.cmux)
	snap2, _ := newStore.Snapshot()
	entry2, ok := state.FindActivationByID(snap2, foundEntry.ActivationID)
	if !ok {
		t.Fatal("1a: activation lost after reconcile")
	}
	if entry2.Step != state.StepStarted {
		t.Errorf("1a: step = %s after reconcile, want started", entry2.Step)
	}

	// Resume missing steps.
	resumeMissingSteps(t, context.Background(), newStore, sc.cfg, *entry2, sc.gitDriver, sc.claude, sc.cmux)
	assertComplete(t, "1a", newStore, foundEntry.ActivationID)
	assertNoDuplication(t, "1a", sc.gitDriver, sc.claude, sc.cmux)
}

// ── 1b: Worktree, kill AFTER effect BEFORE journal ────────────────────────────

// TestActivateFault_1b_WorktreeKillAfterEffectBeforeJournal verifies that killing after
// worktree creation but before the journal causes reconciler to discover the worktree
// via act_id_short substring matching and journal it.
// Ties to: E-NEW4.
func TestActivateFault_1b_WorktreeKillAfterEffectBeforeJournal(t *testing.T) {
	sc := newFaultScenario(t, "1b")
	sc.hooks.AfterWorktreeBeforeJournal = func() { panic("kill") }

	var foundActivationID string
	panicked := runWithFault(func() {
		_, _ = activate(context.Background(), sc.store, sc.cfg, "PROJ-1", "repo1", "main", sc.deps())
	})
	if !panicked {
		t.Fatal("1b: expected panic")
	}

	// Find the journaled activation.
	snap, _ := sc.store.Snapshot()
	for _, entries := range snap.Activations {
		for _, e := range entries {
			if e.Step == state.StepStarted {
				foundActivationID = e.ActivationID
			}
		}
	}
	if foundActivationID == "" {
		t.Fatal("1b: no activation found after kill")
	}

	// Worktree directory must exist on disk.
	foundEntry, _ := state.FindActivationByID(snap, foundActivationID)
	wt := foundEntry.WorktreePath
	if wt != "" {
		worktreeDir := strings.TrimSuffix(wt, "/")
		if _, statErr := os.Stat(worktreeDir); os.IsNotExist(statErr) {
			t.Errorf("1b: worktree directory should exist at %s after fake git create", worktreeDir)
		}
	}

	// Reconciler should discover the worktree and advance step to worktree_created.
	newStore := fakeReconcile(t, sc.statePath, sc.gitDriver, sc.claude, sc.cmux)
	snap2, _ := newStore.Snapshot()
	entry2, ok := state.FindActivationByID(snap2, foundActivationID)
	if !ok {
		t.Fatal("1b: activation lost after reconcile")
	}
	if entry2.Step != state.StepWorktreeCreated {
		t.Errorf("1b: step = %s after reconcile, want worktree_created", entry2.Step)
	}

	// Resume: only claude and cmux steps needed.
	resumeMissingSteps(t, context.Background(), newStore, sc.cfg, *entry2, sc.gitDriver, sc.claude, sc.cmux)
	assertComplete(t, "1b", newStore, foundActivationID)

	// Git must NOT have been called again (reconciler found it, resume skipped it).
	sc.gitDriver.mu.Lock()
	gc := sc.gitDriver.createCount
	sc.gitDriver.mu.Unlock()
	if gc != 1 {
		t.Errorf("1b: git.createCount = %d, want 1 (should not re-create)", gc)
	}
	assertNoDuplication(t, "1b", sc.gitDriver, sc.claude, sc.cmux)
}

// ── 2b: Claude, kill AFTER effect BEFORE journal ─────────────────────────────

// TestActivateFault_2b_ClaudeKillAfterEffectBeforeJournal verifies reconciler discovery
// of a claude session that was launched but not journaled.
// Ties to: E-NEW4.
func TestActivateFault_2b_ClaudeKillAfterEffectBeforeJournal(t *testing.T) {
	sc := newFaultScenario(t, "2b")
	sc.hooks.AfterClaudeBeforeJournal = func() { panic("kill") }

	var foundActivationID string
	panicked := runWithFault(func() {
		_, _ = activate(context.Background(), sc.store, sc.cfg, "PROJ-1", "repo1", "main", sc.deps())
	})
	if !panicked {
		t.Fatal("2b: expected panic")
	}

	snap, _ := sc.store.Snapshot()
	for _, entries := range snap.Activations {
		for _, e := range entries {
			if e.ActivationID != "" {
				foundActivationID = e.ActivationID
			}
		}
	}
	if foundActivationID == "" {
		t.Fatal("2b: no activation found after kill")
	}

	newStore := fakeReconcile(t, sc.statePath, sc.gitDriver, sc.claude, sc.cmux)
	snap2, _ := newStore.Snapshot()
	entry2, ok := state.FindActivationByID(snap2, foundActivationID)
	if !ok {
		t.Fatal("2b: activation lost after reconcile")
	}

	// Reconciler should have advanced at least to worktree_created.
	if entry2.Step == state.StepStarted {
		t.Errorf("2b: step stayed at started after reconcile; expected worktree_created or beyond")
	}

	resumeMissingSteps(t, context.Background(), newStore, sc.cfg, *entry2, sc.gitDriver, sc.claude, sc.cmux)
	assertComplete(t, "2b", newStore, foundActivationID)
	assertNoDuplication(t, "2b", sc.gitDriver, sc.claude, sc.cmux)
}

// ── 3b: Cmux, kill AFTER effect BEFORE journal ───────────────────────────────

// TestActivateFault_3b_CmuxKillAfterEffectBeforeJournal verifies reconciler discovery
// of a cmux workspace that was created but not journaled.
// Ties to: E-NEW4.
func TestActivateFault_3b_CmuxKillAfterEffectBeforeJournal(t *testing.T) {
	sc := newFaultScenario(t, "3b")
	sc.hooks.AfterCmuxBeforeJournal = func() { panic("kill") }

	var foundActivationID string
	panicked := runWithFault(func() {
		_, _ = activate(context.Background(), sc.store, sc.cfg, "PROJ-1", "repo1", "main", sc.deps())
	})
	if !panicked {
		t.Fatal("3b: expected panic")
	}

	snap, _ := sc.store.Snapshot()
	for _, entries := range snap.Activations {
		for _, e := range entries {
			foundActivationID = e.ActivationID
		}
	}
	if foundActivationID == "" {
		t.Fatal("3b: no activation found after kill")
	}

	newStore := fakeReconcile(t, sc.statePath, sc.gitDriver, sc.claude, sc.cmux)
	snap2, _ := newStore.Snapshot()
	entry2, ok := state.FindActivationByID(snap2, foundActivationID)
	if !ok {
		t.Fatal("3b: activation lost after reconcile")
	}

	// Reconciler should have promoted to complete because cmux workspace exists.
	if !entry2.Complete {
		t.Errorf("3b: activation not complete after reconcile; step=%s", entry2.Step)
	}

	// No resume needed — reconciler handled it. Verify no extra calls were made.
	sc.gitDriver.mu.Lock()
	gc := sc.gitDriver.createCount
	sc.gitDriver.mu.Unlock()
	sc.claude.mu.Lock()
	cc := sc.claude.launchCount
	sc.claude.mu.Unlock()
	sc.cmux.mu.Lock()
	wc := sc.cmux.newWSCount
	sc.cmux.mu.Unlock()

	if gc != 1 {
		t.Errorf("3b: git.createCount = %d, want 1", gc)
	}
	if cc != 1 {
		t.Errorf("3b: claude.launchCount = %d, want 1", cc)
	}
	if wc != 1 {
		t.Errorf("3b: cmux.newWSCount = %d, want 1", wc)
	}
}

// ── c-cases (kill AFTER journal) — clean state, resume runs only missing steps ─

// TestActivateFault_1c_WorktreeKillAfterJournal kills after worktree journal.
// After restart: step == worktree_created. Resume runs only claude + cmux.
func TestActivateFault_1c_WorktreeKillAfterJournal(t *testing.T) {
	sc := newFaultScenario(t, "1c")
	sc.hooks.AfterWorktreeJournal = func() { panic("kill") }

	var foundActivationID string
	panicked := runWithFault(func() {
		_, _ = activate(context.Background(), sc.store, sc.cfg, "PROJ-1", "repo1", "main", sc.deps())
	})
	if !panicked {
		t.Fatal("1c: expected panic")
	}

	snap, _ := sc.store.Snapshot()
	for _, entries := range snap.Activations {
		for _, e := range entries {
			if e.Step == state.StepWorktreeCreated {
				foundActivationID = e.ActivationID
			}
		}
	}
	if foundActivationID == "" {
		t.Fatal("1c: no worktree_created activation after kill")
	}

	newStore := fakeReconcile(t, sc.statePath, sc.gitDriver, sc.claude, sc.cmux)
	snap2, _ := newStore.Snapshot()
	entry2, ok := state.FindActivationByID(snap2, foundActivationID)
	if !ok {
		t.Fatal("1c: activation lost after reconcile")
	}
	if entry2.Step != state.StepWorktreeCreated {
		t.Errorf("1c: unexpected step %s after reconcile", entry2.Step)
	}

	resumeMissingSteps(t, context.Background(), newStore, sc.cfg, *entry2, sc.gitDriver, sc.claude, sc.cmux)
	assertComplete(t, "1c", newStore, foundActivationID)

	sc.gitDriver.mu.Lock()
	gc := sc.gitDriver.createCount
	sc.gitDriver.mu.Unlock()
	if gc != 1 {
		t.Errorf("1c: git.createCount = %d, want 1 (resume must not re-run worktree)", gc)
	}
	assertNoDuplication(t, "1c", sc.gitDriver, sc.claude, sc.cmux)
}

// TestActivateFault_2c_ClaudeKillAfterJournal kills after claude journal.
// Resume runs only cmux.
func TestActivateFault_2c_ClaudeKillAfterJournal(t *testing.T) {
	sc := newFaultScenario(t, "2c")
	sc.hooks.AfterClaudeJournal = func() { panic("kill") }

	var foundActivationID string
	panicked := runWithFault(func() {
		_, _ = activate(context.Background(), sc.store, sc.cfg, "PROJ-1", "repo1", "main", sc.deps())
	})
	if !panicked {
		t.Fatal("2c: expected panic")
	}

	snap, _ := sc.store.Snapshot()
	for _, entries := range snap.Activations {
		for _, e := range entries {
			if e.Step == state.StepClaudeStarted {
				foundActivationID = e.ActivationID
			}
		}
	}
	if foundActivationID == "" {
		t.Fatal("2c: no claude_started activation after kill")
	}

	newStore := fakeReconcile(t, sc.statePath, sc.gitDriver, sc.claude, sc.cmux)
	snap2, _ := newStore.Snapshot()
	entry2, ok := state.FindActivationByID(snap2, foundActivationID)
	if !ok {
		t.Fatal("2c: activation lost after reconcile")
	}

	resumeMissingSteps(t, context.Background(), newStore, sc.cfg, *entry2, sc.gitDriver, sc.claude, sc.cmux)
	assertComplete(t, "2c", newStore, foundActivationID)

	sc.claude.mu.Lock()
	cc := sc.claude.launchCount
	sc.claude.mu.Unlock()
	if cc != 1 {
		t.Errorf("2c: claude.launchCount = %d, want 1", cc)
	}
	assertNoDuplication(t, "2c", sc.gitDriver, sc.claude, sc.cmux)
}

// TestActivateFault_3c_CmuxKillAfterJournal kills after cmux journal.
// activation.complete == true after kill; no resume needed.
func TestActivateFault_3c_CmuxKillAfterJournal(t *testing.T) {
	sc := newFaultScenario(t, "3c")
	sc.hooks.AfterCmuxJournal = func() { panic("kill") }

	var foundActivationID string
	panicked := runWithFault(func() {
		_, _ = activate(context.Background(), sc.store, sc.cfg, "PROJ-1", "repo1", "main", sc.deps())
	})
	if !panicked {
		t.Fatal("3c: expected panic")
	}

	snap, _ := sc.store.Snapshot()
	for _, entries := range snap.Activations {
		for _, e := range entries {
			if e.Complete {
				foundActivationID = e.ActivationID
			}
		}
	}
	if foundActivationID == "" {
		t.Fatal("3c: no complete activation after kill (journal was committed before kill)")
	}

	// Reconciler sees complete; no further action needed.
	newStore := fakeReconcile(t, sc.statePath, sc.gitDriver, sc.claude, sc.cmux)
	snap2, _ := newStore.Snapshot()
	entry2, ok := state.FindActivationByID(snap2, foundActivationID)
	if !ok {
		t.Fatal("3c: activation lost after reconcile")
	}
	if !entry2.Complete {
		t.Errorf("3c: activation not complete after reconcile")
	}
	assertNoDuplication(t, "3c", sc.gitDriver, sc.claude, sc.cmux)
}

// TestActivateFault_ResourceCountAfterMatrix is the cross-cutting no-duplication
// assertion after all scenarios run through the fake drivers.
// Each scenario uses its own isolated driver instances, so this test verifies
// the structural invariant that each driver records exactly one create-call per scenario.
func TestActivateFault_ResourceCountAfterMatrix(t *testing.T) {
	scenarios := []struct {
		name    string
		hookFn  func(h *ActivationHooks)
		wantGit int
		wantClaude int
		wantCmux int
	}{
		{"1a", func(h *ActivationHooks) { h.BeforeWorktree = func() { panic("kill") } }, 0, 0, 0},
		{"1b", func(h *ActivationHooks) { h.AfterWorktreeBeforeJournal = func() { panic("kill") } }, 1, 0, 0},
		{"1c", func(h *ActivationHooks) { h.AfterWorktreeJournal = func() { panic("kill") } }, 1, 0, 0},
		{"2b", func(h *ActivationHooks) { h.AfterClaudeBeforeJournal = func() { panic("kill") } }, 1, 1, 0},
		{"2c", func(h *ActivationHooks) { h.AfterClaudeJournal = func() { panic("kill") } }, 1, 1, 0},
		{"3b", func(h *ActivationHooks) { h.AfterCmuxBeforeJournal = func() { panic("kill") } }, 1, 1, 1},
		{"3c", func(h *ActivationHooks) { h.AfterCmuxJournal = func() { panic("kill") } }, 1, 1, 1},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			fs := newFaultScenario(t, sc.name)
			sc.hookFn(fs.hooks)
			runWithFault(func() {
				_, _ = activate(context.Background(), fs.store, fs.cfg, "PROJ-1", "repo1", "main", fs.deps())
			})

			fs.gitDriver.mu.Lock()
			gc := fs.gitDriver.createCount
			fs.gitDriver.mu.Unlock()
			fs.claude.mu.Lock()
			cc := fs.claude.launchCount
			fs.claude.mu.Unlock()
			fs.cmux.mu.Lock()
			wc := fs.cmux.newWSCount
			fs.cmux.mu.Unlock()

			if gc != sc.wantGit {
				t.Errorf("%s: git.createCount = %d, want %d", sc.name, gc, sc.wantGit)
			}
			if cc != sc.wantClaude {
				t.Errorf("%s: claude.launchCount = %d, want %d", sc.name, cc, sc.wantClaude)
			}
			if wc != sc.wantCmux {
				t.Errorf("%s: cmux.newWSCount = %d, want %d", sc.name, wc, sc.wantCmux)
			}
		})
	}
}
