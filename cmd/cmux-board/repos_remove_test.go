package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

// writeConfigWithRepos writes a config.json containing the given repos to dir.
func writeConfigWithRepos(t *testing.T, dir string, repos map[string]config.RepoEntry) {
	t.Helper()
	cfg := config.Config{
		SchemaVersion: config.SchemaVersionCurrent,
		Repos:         repos,
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// writeStateWithTickets writes a state.json to dir.
func writeStateWithTickets(t *testing.T, dir string, tickets map[string]state.TicketState, activations map[string][]state.ActivationEntry) {
	t.Helper()
	if tickets == nil {
		tickets = make(map[string]state.TicketState)
	}
	if activations == nil {
		activations = make(map[string][]state.ActivationEntry)
	}
	st := state.State{
		SchemaVersion: state.SchemaVersionCurrent,
		Tickets:       tickets,
		Activations:   activations,
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.StateFileName), data, 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}
}

func TestReposRemoveCmd_HappyPath(t *testing.T) {
	configDir := setupTempConfigDir(t)
	writeConfigWithRepos(t, configDir, map[string]config.RepoEntry{
		"foo": {Name: "Foo", Path: "/tmp/foo", DefaultBranch: "main"},
	})
	writeStateWithTickets(t, configDir, nil, nil)

	cmd := newReposRemoveCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.RunE(cmd, []string{"foo"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	if !strings.Contains(buf.String(), "foo") {
		t.Errorf("stdout missing 'foo'; got: %s", buf.String())
	}

	repos := readConfigRepos(t, configDir)
	if _, ok := repos["foo"]; ok {
		t.Error("expected 'foo' to be removed from config.json")
	}
}

func TestReposRemoveCmd_RefusedTicketAssignment(t *testing.T) {
	configDir := setupTempConfigDir(t)
	writeConfigWithRepos(t, configDir, map[string]config.RepoEntry{
		"foo": {Name: "Foo"},
	})
	writeStateWithTickets(t, configDir,
		map[string]state.TicketState{
			"PROJ-42": {Key: "PROJ-42", AssignedRepoIDs: []string{"foo"}},
		},
		nil,
	)

	beforeBytes := configBytes(t, configDir)

	cmd := newReposRemoveCmd()
	err := cmd.RunE(cmd, []string{"foo"})
	if !errors.Is(err, config.ErrRepoReferenced) {
		t.Errorf("expected ErrRepoReferenced, got %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "PROJ-42") {
		t.Errorf("error should mention PROJ-42, got: %v", err)
	}

	afterBytes := configBytes(t, configDir)
	if string(beforeBytes) != string(afterBytes) {
		t.Error("config.json changed despite refusal")
	}
}

func TestReposRemoveCmd_RefusedActivation(t *testing.T) {
	configDir := setupTempConfigDir(t)
	writeConfigWithRepos(t, configDir, map[string]config.RepoEntry{
		"foo": {Name: "Foo"},
	})
	writeStateWithTickets(t, configDir,
		map[string]state.TicketState{
			"PROJ-42": {Key: "PROJ-42"},
		},
		map[string][]state.ActivationEntry{
			"PROJ-42": {{ActivationID: "act-1", RepoID: "foo"}},
		},
	)

	beforeBytes := configBytes(t, configDir)

	cmd := newReposRemoveCmd()
	err := cmd.RunE(cmd, []string{"foo"})
	if !errors.Is(err, config.ErrRepoReferenced) {
		t.Errorf("expected ErrRepoReferenced, got %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "PROJ-42") {
		t.Errorf("error should mention PROJ-42, got: %v", err)
	}

	afterBytes := configBytes(t, configDir)
	if string(beforeBytes) != string(afterBytes) {
		t.Error("config.json changed despite refusal")
	}
}

func TestReposRemoveCmd_AssignedTicket(t *testing.T) {
	configDir := setupTempConfigDir(t)
	writeConfigWithRepos(t, configDir, map[string]config.RepoEntry{
		"bar": {Name: "Bar"},
	})
	writeStateWithTickets(t, configDir,
		map[string]state.TicketState{
			"PROJ-99": {Key: "PROJ-99", Source: "jira", AssignedRepoIDs: []string{"bar"}},
		},
		nil,
	)

	cmd := newReposRemoveCmd()
	err := cmd.RunE(cmd, []string{"bar"})
	if !errors.Is(err, config.ErrRepoReferenced) {
		t.Errorf("expected ErrRepoReferenced for assigned ticket, got %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "PROJ-99") {
		t.Errorf("error should mention PROJ-99, got: %v", err)
	}
}

func TestReposRemoveCmd_NotFound(t *testing.T) {
	configDir := setupTempConfigDir(t)
	writeConfigWithRepos(t, configDir, map[string]config.RepoEntry{})
	writeStateWithTickets(t, configDir, nil, nil)

	beforeBytes := configBytes(t, configDir)

	cmd := newReposRemoveCmd()
	err := cmd.RunE(cmd, []string{"ghost"})
	if err == nil {
		t.Fatal("expected error for unknown repo_id, got nil")
	}

	afterBytes := configBytes(t, configDir)
	if string(beforeBytes) != string(afterBytes) {
		t.Error("config.json changed for not-found error")
	}
}

func TestReposRemoveCmd_StateAbsent(t *testing.T) {
	configDir := setupTempConfigDir(t)
	writeConfigWithRepos(t, configDir, map[string]config.RepoEntry{
		"foo": {Name: "Foo"},
	})
	// No state.json written — state.Open returns empty DefaultState.

	cmd := newReposRemoveCmd()
	if err := cmd.RunE(cmd, []string{"foo"}); err != nil {
		t.Fatalf("expected success with absent state.json, got: %v", err)
	}

	repos := readConfigRepos(t, configDir)
	if _, ok := repos["foo"]; ok {
		t.Error("expected 'foo' removed")
	}
}
