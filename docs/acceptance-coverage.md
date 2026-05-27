# Acceptance Criteria Coverage

One row per criterion. Status: **covered** = at least one passing test exercises the scenario;
**partial** = mechanics tested but the full end-to-end scenario is not exercised in a single test;
**gap** = no automated test; **deferred** = planned for a later milestone.

## Functional Criteria

| # | Criterion (short) | Test file(s) | Test function(s) | Status |
|---|---|---|---|---|
| F1 | `init` wizard writes config+state+credentials with correct mode bits | `internal/config/credentials_test.go` `internal/config/modebits_test.go` `internal/initwizard/finalize_test.go` | `TestCredentialsFileModeAfterSave` `TestConfigFileModeAfterSave` `TestValidateCredentialsModeOK` `TestEnsureConfigDirModeOK` `TestFinalize_HappyPath` | partial (unit-level mode bit + wizard finalize; no single end-to-end test hits all three files + mode assertions together) |
| F2 | `init --force` overwrites config+credentials but preserves state.json | `cmd/cmux-board/init_cmd_force_reset_test.go` | `TestInitCmd_ForcePreservesStateJSON` | covered |
| F3 | `init --reset` removes all three files | `cmd/cmux-board/init_cmd_force_reset_test.go` | `TestInitCmd_ResetRemovesAllThreeFiles` `TestInitCmd_ResetToleratesMissingFiles` | covered |
| F4 | `init --api-token-stdin` reads token from stdin; no token in argv | `cmd/cmux-board/init_cmd_argv_test.go` | `TestInitCmd_NoAPITokenFlag` `TestInitCmd_APITokenStdinFlagRegistered` `TestReadToken_ArgvSnapshotDuringStdinRead` | covered |
| F5 | Startup refuses if credentials.json looser than 0600 (without --unsafe-creds) | `internal/config/modebits_test.go` | `TestValidateCredentialsModeUnsafe` `TestValidateCredentialsModeUnsafeFlag` | covered |
| F6 | Startup refuses if `cmux --json identify` fails | `cmd/cmux-board/dock_cmd_test.go` | `TestDockPreflightCmuxFailAborts` | covered |
| F7 | Startup refuses if claude version < 2.1.150 | `cmd/cmux-board/dock_cmd_test.go` `internal/claudecli/version_test.go` | `TestDockPreflightClaudeVersionFailAborts` `TestMeetsMin` | covered |
| F8 | Startup refuses if `CLAUDE_CODE_DISABLE_AGENT_VIEW=1` | `internal/claudecli/disable_check_test.go` | `TestCheckDisableAgentView_EnvSet` `TestCheckDisableAgentView_EnvSetTrue` | covered |
| F9 | `dock` renders mirrored board with columns and unmapped column | `internal/ui/view_test.go` `internal/sync/unmapped_test.go` | `TestView_ThreeTicketsTwoColumnsUnmapped` `TestView_EmptyBoard` `TestResolve_OneUnmapped` `TestResolve_AllMapped` | partial (view + resolve tested separately; no golden-diff screenshot test) |
| F10 | Poller fires at poll_interval; < 10s clamped to 10s with warning | `internal/sync/poller_test.go` | `TestPoller_FiresAtInterval` `TestPoller_IntervalClamped` | covered |
| F11 | Card move triggers `TransitionStatus`; `last_known_status` updated on success | `internal/sync/push_test.go` `internal/sync/bridge_test.go` | `TestPush_HappyPath` `TestBridgePushOKMsg` | covered |
| F12 | `ErrConflict` snaps card back; non-modal toast rendered | `internal/sync/push_test.go` `internal/sync/bridge_test.go` `internal/ui/update_test.go` | `TestPush_ErrConflictBare` `TestPush_ErrConflictWithWrapper` `TestBridgePushConflictMsg` `TestUpdate_PollErrSetsState` | covered |
| F13 | Enter with 0 activations: worktree + claude cmd.Dir (no --cwd) + cmux workspace; persists activation | `internal/ui/activate_test.go` `internal/claudecli/bg_test.go` | `TestActivate_StepProgressionOnSuccess` `TestActivate_NegativeArgv_NoCwd` `TestLaunchBackground_CmdDirSet` `TestLaunchBackground_NegativeArgv_NoCwd` `TestActivate_ActIDShortEncoding` | covered |
| F14 | Enter with 1 activation focuses via `cmux focus-pane`; no bare `focus` subcommand | `internal/ui/focus_test.go` `internal/cmuxcli/focus_pane_test.go` | `TestFocus_HealthyActivation` `TestFocus_NegativeArgv_FocusPaneNotBareFocus` `TestFocusPane_HappyPath` `TestFocusPane_NegativeArgv_NoBareFreeFocus` | covered |
| F15 | Enter with n>1 activations opens picker; Enter/n/r/d/Esc keys per spec | `internal/ui/picker_test.go` `internal/ui/keymap_test.go` | `TestNewPickerState_NonNilForTwo` `TestPickerCursorClamping` `TestPickerDelete_RemovesEntry` `TestHandlePickerMode_EscClearsState` | partial (Esc + delete + cursor tested; `r` respawn key path and Enter-confirm within picker not directly tested) |
| F16 | Capital `N` on any ticket creates new activation regardless of prior count | `internal/ui/keymap_test.go` `internal/ui/model_test.go` | `TestHandleNormalMode_NewApproach` `TestNewModel_ApproachInputReady` | partial (key dispatch + state init tested; full new-activation flow on pre-existing activations not in a single test) |
| F17 | `state.json` writes are atomic; kill mid-write preserves prior version | `internal/config/atomic_test.go` `internal/state/race_test.go` | `TestWriteFileAtomicCreatesFile` `TestWriteFileAtomicRoundTrip` `TestWriteFileAtomicRenameFailure` `TestPollVsActivateParallelism` | covered |
| F18 | Persistence schema: no Jira-specific fields outside `tickets[*].raw` and `config.adapter_config` | `internal/state/state_test.go` `internal/sync/merge_test.go` | `TestStateJSONRoundTrip` `TestMergePulledTickets_TrackerFieldsOverwritten` | partial (no static schema field-name check; struct-level coverage only) |
| F19 | `IssueTracker` interface has exactly the listed methods; only Jira adapter compiled | `internal/tracker/jira/capabilities_test.go` | `TestJiraAdapterCapabilities` | partial (capabilities tested; no `go doc` / `go list` static check in automated test) |
| F20 | Token redaction at `secretsink.Writer` stage: direct attr, error, Stringer, LogValuer, Group | `internal/secretsink/security_test.go` `internal/runtime/logger_test.go` | `TestF20a_RawTokenViaSlogAttr` `TestF20f_WrappedErrorViaAny` `TestF20g_CustomStringer` `TestF20h_LogValuer` `TestF20i_SlogGroupDirect` `TestF20e_Meta_NakedBufferDoesNotRedact` | covered |
| F-NEW | cmux-board never invokes destructive cmux commands | `verify_additive_test.go` `internal/cmuxcli/package_invariants_test.go` `scripts/check_additive_only.sh` | `TestAdditiveOnly` `TestPackageInvariants_NoDestructiveTokens` | covered |
| F-MR1 | `repos add` registers repo; `repos list` shows it; persists across restart | `cmd/cmux-board/repos_add_test.go` `cmd/cmux-board/repos_list_test.go` `internal/config/repos_test.go` | `TestReposAddCmd_HappyPath` `TestReposListCmd_OneRepo` `TestReposListCmd_JSONOutput` `TestAddRepo_HappyPath` | covered |
| F-MR2 | Activating unassigned ticket prompts repo picker; chosen repo stored in assigned_repo_ids | `internal/ui/resolve_repo_test.go` | `TestResolveRepoAndRoute_ZeroAssignedOpensPicker` `TestHandleRepoPickerMode_FirstTouchSelectAppendsAssignment` | covered |
| F-MR3 | Ticket with 2+ assigned repos shows per-repo picker; selects correct repo_id | `internal/ui/resolve_repo_test.go` | `TestResolveRepoAndRoute_MultiAssignedOpensPicker` | partial (picker shown tested; full activation with specific repo_id not in a single test) |
| F-MR4 | Picker key `a` opens assignment editor; toggle + commit updates state atomically | `internal/ui/assignment_editor_test.go` `internal/ui/view_test.go` | `TestHandleAssignmentEditor_CommitAtomic` `TestHandleAssignmentEditor_CommitWritesStore` `TestView_AssignmentEditorOverlay` | covered |
| F-MR5 | `repos remove` refuses if ticket has id in assigned_repo_ids or any activation references it | `cmd/cmux-board/repos_remove_test.go` `internal/config/repos_test.go` | `TestReposRemoveCmd_RefusedTicketAssignment` `TestReposRemoveCmd_RefusedActivation` `TestRemoveRepo_RefusedByTicketAssignment` `TestRemoveRepo_RefusedByActivation` | covered |
| F-DR1 | `config.DryRun: true` short-circuits Push; no TransitionStatus call; state unchanged | `internal/sync/push_test.go` | `TestPush_DryRun_NoTrackerCall` `TestPush_DryRun_NoStateMutation` `TestPush_DryRun_ServerStatusEqualsExpectedFrom` `TestPush_DryRun_UnknownTicketStillErrors` | covered |
| F-CLEAN | Repo presents as clean cmux-board project; zero OpenKanban references | — | — | deferred to M-012 (T-087 covers README/AGENTS/CLAUDE.md; full grep gate is M-012 scope) |

## Edge Case Criteria

| # | Criterion (short) | Test file(s) | Test function(s) | Status |
|---|---|---|---|---|
| E1 | Unmapped status renders in synthetic Unmapped column with `?` badge; column appears only when populated | `internal/ui/view_test.go` `internal/sync/unmapped_test.go` | `TestView_ThreeTicketsTwoColumnsUnmapped` `TestResolve_OneUnmapped` `TestResolve_AllUnmapped` | partial (render + resolve tested; `?` badge and column-disappear-when-empty not in a single integration test) |
| E2 | Push from Unmapped column uses `last_known_status` as `expected_from_status` | — | — | gap (no test asserts the unmapped-specific expected_from payload; `TestPush_HappyPath` covers the field but not from-unmapped scenario) |
| E3 | Ticket renamed/re-statused mid-poll lands cleanly; `last_known_status` updated; no double-transition | `internal/sync/merge_test.go` | `TestMergePulledTickets_E3Sequence` | covered |
| E4 | 5xx triggers exp backoff 60s→2m→5m; reset on first success; tracker pill reflects offline state | `internal/sync/backoff_test.go` `internal/sync/poller_test.go` `internal/cmuxcli/set_status_test.go` | `TestBackoffSequence` `TestBackoffCapMaintained` `TestBackoffReset` `TestPoller_5xxError` `TestSetStatus_TrackerOffline_UsesOfflineColor` | partial (backoff sequence + poller 5xx tested; pill timestamp text not in a single end-to-end test) |
| E5 | 401 shows distinct pill text "re-auth required — run cmux-board init --force"; pushes blocked | `internal/sync/events_test.go` `internal/ui/pills_test.go` `internal/sync/poller_test.go` | `TestPollErrMsgPillTextAuth` `TestPollErr_401_MapsToReauth` `TestPoller_401Error` | covered |
| E6 | cmux socket failure at runtime retries 3x at 200ms; persistent failure sets pill; board keeps rendering | `internal/cmuxcli/runner_test.go` `internal/cmuxcli/set_status_test.go` | `TestRunCmux_RetrySucceedThirdAttempt` `TestRunCmux_ExhaustionReturnsErrCmuxUnreachable` `TestRunCmux_SocketBrokenPatternTriggersRetry` `TestSetStatus_CmuxUnreachable_UsesRedColor` | partial (retry timing + pill tested; "board keeps rendering" runtime behavior not in automated test) |
| E7 | Stale `claude_short_id` → activation marked `claude_orphan: true`; picker shows `[orphan]`; `r` respawns; `d` removes | `internal/claudecli/respawn_test.go` `internal/ui/picker_test.go` `internal/ui/focus_test.go` | `TestIsOrphan_SessionAbsent` `TestRenderPicker_OrphanGlyph` `TestFocus_ClaudeOrphan_ReturnNeedsRespawn` | partial (orphan detection + glyph + focus path tested; `r` key full respawn flow in picker not in a single test) |
| E8 | Stale `cmux_workspace_id` → activation marked `cmux_orphan: true`; focus creates new workspace; foreign workspace left untouched | `internal/ui/focus_test.go` `internal/cmuxcli/list_workspaces_test.go` | `TestFocus_CmuxOrphan_CreatesNewWorkspace_NoForeignAdoption` `TestNoCurrentDirectoryAdoption` | covered |
| E-NEW2 | Pre-existing plain directory at target path → activation uniquifies with `-2` suffix; plain dir and sentinel untouched | `internal/ui/uniquify_test.go` `internal/ui/activate_test.go` `internal/git/createworktreeat_test.go` | `TestUniquify_FirstSlotTaken` `TestUniquify_SuffixContentsUnchanged_Sentinel` `TestActivate_SuffixAppliedToPathAndBranchNotClaudeName` `TestCreateWorktreeAt` | covered |
| F-NEW3 | ULID `activation_id` journaled before side effects; `act_id_short` embedded in all resource names; incremental journaling | `internal/ui/activate_test.go` | `TestActivate_ActIDShortEncoding` `TestActivate_JournalsBeforeFirstSideEffect` `TestActivate_StepProgressionOnSuccess` | covered |
| F-NEW4 | Startup reconciliation discovers resources by `act_id_short` substring; harvests IDs; promotes to `complete: true` | `internal/runtime/reconcile_test.go` `cmd/cmux-board/dock_cmd_test.go` | `TestReconcile_PromotesWhenAllFound` `TestReconcile_PartialHarvest_OneOfThree` `TestReconcile_ZeroFound` `TestDockStartupReconciliationCalled` | covered |
| E-NEW3 | Kill between journaled steps → startup shows `[incomplete]`; picker `r` completes only missing steps | `internal/ui/activate_fault_test.go` `internal/runtime/reconcile_test.go` | `TestActivateFault_2c_ClaudeKillAfterJournal` `TestActivateFault_3c_CmuxKillAfterJournal` `TestResumeActivation_SkipsAlreadyCreatedSteps` | covered |
| E-NEW4 | Kill after side effect but before journal → reconciler discovers resource; picker `r` does not recreate | `internal/ui/activate_fault_test.go` | `TestActivateFault_1b_WorktreeKillAfterEffectBeforeJournal` `TestActivateFault_2b_ClaudeKillAfterEffectBeforeJournal` `TestActivateFault_3b_CmuxKillAfterEffectBeforeJournal` `TestActivateFault_ResourceCountAfterMatrix` | covered |
| F-NEW5 | Poll merge preserves `assigned_repo_ids` on existing tickets; only tracker-owned fields overwritten | `internal/sync/merge_test.go` `internal/state/state_test.go` `internal/state/race_test.go` | `TestMergePulledTickets_TrackerFieldsOverwritten` `TestAssignedRepoIDsPreserved` `TestPollVsActivateParallelism` | covered |
| E-NEW5 | Removed ticket gets `removed_at` stamp; hidden in UI; reappears when polled again; `repos remove` still blocks on it | `internal/sync/merge_test.go` `cmd/cmux-board/repos_remove_test.go` `internal/sync/unmapped_test.go` | `TestMergePulledTickets_RemovedAtStampedOnce` `TestMergePulledTickets_ReappearedTicketClearsRemovedAt` `TestReposRemoveCmd_RemovedAtTicket` `TestResolve_RemovedTicketsExcluded` | partial (stamp + re-appear + repos-remove tested; UI hide behavior not in automated test) |
| E9 | Dirty worktree: no warning, no block, no `git status` | — | — | gap (no test asserts `git status` is never called; additive grep does not cover git commands) |
| E10 | claude < 2.1.150 → refuse with actionable error | `internal/claudecli/version_test.go` `cmd/cmux-board/dock_cmd_test.go` | `TestMeetsMin` `TestDockPreflightClaudeVersionFailAborts` | covered |
| E11 | `CLAUDE_CODE_DISABLE_AGENT_VIEW=1` → refuse with actionable error | `internal/claudecli/disable_check_test.go` | `TestCheckDisableAgentView_EnvSet` `TestCheckDisableAgentView_EnvSetTrue` | covered |
| E12 | credentials.json at 0644 without --unsafe-creds: refuse; with --unsafe-creds: warn but start | `internal/config/modebits_test.go` | `TestValidateCredentialsModeUnsafe` `TestValidateCredentialsModeUnsafeFlag` | covered |
| E13 | `init --force` against existing valid config preserves state.json byte-for-byte | `cmd/cmux-board/init_cmd_force_reset_test.go` | `TestInitCmd_ForcePreservesStateJSON` | covered |
| E14 | Approach name with mixed case/special chars → path sanitized to lowercase+dashes; original stored | `internal/git/worktree_test.go` | `TestSanitizeBranchName` | partial (branch sanitize tested; no test for approach-name display vs path comparison in activation) |
| E15 | Jira workflow with `RequiresWorkflowID=true`: adapter reads transition IDs from `ticket.raw`; push uses correct ID | `internal/tracker/jira/transitions_test.go` `internal/tracker/jira/transition_test.go` | `TestTransitionIDFromRawCaseInsensitive` `TestTransitionIDFromRawMiss` `TestTransitionIDFromRawNilMap` `TestTransitionStatusPOST400WorkflowForbidden` | partial (cache helpers tested; no test drives a full Requires WorkflowID=true push payload from board card) |
| E-NEW | Foreign workspace at our target path on new activation → uniquify path; foreign workspace untouched | `internal/ui/activate_test.go` `internal/ui/uniquify_test.go` | `TestActivate_SuffixAppliedToPathAndBranchNotClaudeName` `TestUniquify_FirstSlotTaken` | partial (uniquify mechanic tested; no single test pre-seeds a foreign cmux workspace and asserts it is left untouched during activation) |
| E-MR1 | Stale repo ID in assigned_repo_ids → toast + `[?]` badge; assignment editor flags stale ID | `internal/ui/resolve_repo_test.go` `internal/ui/assignment_editor_test.go` | `TestHandleRepoPickerMode_StaleIdCancelsWithToast` `TestNewAssignmentEditorState_StaleRepoPresentInList` | covered |
| E-MR2 | `repos add <non-git-path>` refused; config.json unchanged | `cmd/cmux-board/repos_add_test.go` `internal/config/repo_validate_test.go` | `TestReposAddCmd_NonGitPath` `TestValidateRepoPath_Missing` | covered |
| E-MR3 | Registered repo path deleted → activation surfaces per-action toast; assigned_repo_ids not auto-cleared | — | — | gap (no test pre-registers a repo then deletes its path and drives activation) |
| E-MR4 | Worktree path collision on disk → uniquify with `-2`/`-3` counter | `internal/ui/uniquify_test.go` `internal/ui/activate_test.go` | `TestUniquify_TwoSlotsTaken` `TestUniquify_ThreeSlotsTaken` `TestActivate_SuffixParallelism` | covered |

## F20 Sub-Criteria

| # | Criterion | Test file(s) | Test function(s) | Status |
|---|---|---|---|---|
| F20.a | Direct slog attribute containing raw token is redacted | `internal/secretsink/security_test.go` `internal/runtime/logger_test.go` | `TestF20a_RawTokenViaSlogAttr` `TestF20a` | covered |
| F20.d | No HTTP Authorization header logging during Jira requests | `internal/tracker/jira/security_test.go` | `TestF20d_NoHeaderLoggingDuringRequest` | covered |
| F20.e | Meta sanity: naked buffer logs raw token; bad fixture logs header | `internal/secretsink/security_test.go` `internal/tracker/jira/security_test.go` | `TestF20e_Meta_NakedBufferDoesNotRedact` `TestF20e_Meta_HeaderLeakIsDetectable` | covered |
| F20.f | Token inside `fmt.Errorf` wrapped error passed as `slog.Any` is redacted | `internal/secretsink/security_test.go` `internal/runtime/logger_test.go` | `TestF20f_WrappedErrorViaAny` `TestF20f` | covered |
| F20.g | Token returned by `fmt.Stringer.String()` is redacted | `internal/secretsink/security_test.go` `internal/runtime/logger_test.go` | `TestF20g_CustomStringer` `TestF20g` | covered |
| F20.h | Token inside `slog.LogValuer` Group is redacted | `internal/secretsink/security_test.go` | `TestF20h_LogValuer` | covered |
| F20.i | Token inside `slog.Group` direct attrs is redacted | `internal/secretsink/security_test.go` `internal/runtime/logger_test.go` | `TestF20i_SlogGroupDirect` `TestF20i` | covered |

## Coverage Summary

| Status | Count |
|---|---|
| covered | 28 |
| partial | 14 |
| gap | 4 |
| deferred | 1 |

## Gaps Requiring Discovered Bundles

The following gaps are non-trivial and each warrants a discovered bundle:

- **E2**: No test verifies that a push from the Unmapped column sends `last_known_status` as the OCC `expected_from_status`. The push handler code path may not set this correctly.
- **E9**: No test asserts `git status` is never called on focus of a dirty worktree. Requires a test that mocks the git runner and asserts no `git status` invocation.
- **E-MR3**: No test drives the scenario where a registered repo's on-disk path is deleted and then activation is attempted. Missing path should produce a per-action toast without clearing `assigned_repo_ids`.

F-CLEAN is tracked as deferred to M-012 (a seventh grep arm in `check_additive_only.sh` plus a full visual review).
