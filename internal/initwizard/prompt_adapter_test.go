package initwizard

import (
	"bytes"
	"strings"
	"testing"
)

func TestPromptAdapterPick_DefaultEnter(t *testing.T) {
	r := strings.NewReader("\n")
	var w bytes.Buffer
	choice, err := PromptAdapterPick(r, &w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice.AdapterName != adapterNameJira {
		t.Errorf("expected %q, got %q", adapterNameJira, choice.AdapterName)
	}
}

func TestPromptAdapterPick_SelectOne(t *testing.T) {
	r := strings.NewReader("1\n")
	var w bytes.Buffer
	choice, err := PromptAdapterPick(r, &w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice.AdapterName != adapterNameJira {
		t.Errorf("expected %q, got %q", adapterNameJira, choice.AdapterName)
	}
}

func TestPromptAdapterPick_SelectJira(t *testing.T) {
	r := strings.NewReader("jira\n")
	var w bytes.Buffer
	choice, err := PromptAdapterPick(r, &w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice.AdapterName != adapterNameJira {
		t.Errorf("expected %q, got %q", adapterNameJira, choice.AdapterName)
	}
}

func TestPromptAdapterPick_InvalidThenValid(t *testing.T) {
	// First attempt is invalid, second is "1"
	r := strings.NewReader("99\n1\n")
	var w bytes.Buffer
	choice, err := PromptAdapterPick(r, &w)
	if err != nil {
		t.Fatalf("unexpected error after second attempt: %v", err)
	}
	if choice.AdapterName != adapterNameJira {
		t.Errorf("expected %q, got %q", adapterNameJira, choice.AdapterName)
	}
	if !strings.Contains(w.String(), "Invalid selection") {
		t.Errorf("expected re-prompt message, got: %q", w.String())
	}
}

func TestPromptAdapterPick_MaxRetriesExhausted(t *testing.T) {
	// Provide 5 invalid inputs, all "99"
	inputs := strings.Repeat("99\n", maxAdapterPickRetries)
	r := strings.NewReader(inputs)
	var w bytes.Buffer
	_, err := PromptAdapterPick(r, &w)
	if err == nil {
		t.Fatal("expected error after max retries, got nil")
	}
}

func TestPromptAdapterPick_MenuContainsPlaceholder(t *testing.T) {
	r := strings.NewReader("1\n")
	var w bytes.Buffer
	_, err := PromptAdapterPick(r, &w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(w.String(), "more adapters coming") {
		t.Errorf("expected placeholder line in menu output, got: %q", w.String())
	}
}
