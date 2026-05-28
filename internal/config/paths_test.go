package config

import (
	"strings"
	"testing"
)

func TestResolveCredentialsPathFlag(t *testing.T) {
	t.Parallel()
	got, err := ResolveCredentialsPath("/custom/path/creds.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/custom/path/creds.json" {
		t.Errorf("got %q, want /custom/path/creds.json", got)
	}
}

func TestResolveCredentialsPathEnv(t *testing.T) {
	t.Setenv(EnvCredentialsPath, "/tmp/creds.json")
	got, err := ResolveCredentialsPath("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/creds.json" {
		t.Errorf("got %q, want /tmp/creds.json", got)
	}
}

func TestResolveCredentialsPathDefault(t *testing.T) {
	t.Setenv(EnvCredentialsPath, "") // ensure env is cleared
	got, err := ResolveCredentialsPath("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(got, CredentialsFileName) {
		t.Errorf("default path should end with %q, got %q", CredentialsFileName, got)
	}
	if !strings.Contains(got, DefaultConfigDir) {
		t.Errorf("default path should contain %q, got %q", DefaultConfigDir, got)
	}
}

func TestFlagOverridesEnv(t *testing.T) {
	t.Setenv(EnvCredentialsPath, "/tmp/env-creds.json")
	got, err := ResolveCredentialsPath("/flag/value/creds.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/flag/value/creds.json" {
		t.Errorf("flag should override env: got %q, want /flag/value/creds.json", got)
	}
}
