package secretsink

import (
	"bytes"
	"strings"
	"testing"
)

// resetSecrets clears the registered secrets for test isolation.
// Called at the start of each test that registers secrets.
func resetSecrets() {
	mu.Lock()
	defer mu.Unlock()
	secrets = nil
}

func TestRedactBytesRegistered(t *testing.T) {
	resetSecrets()
	token := "mysecrettoken12345"
	Register(token)
	input := []byte("log line containing mysecrettoken12345 in it")
	out := RedactBytes(input)
	if bytes.Contains(out, []byte(token)) {
		t.Errorf("token still present in output: %q", out)
	}
	// For token > 9 chars, replacement is first5...last4
	// "mysecrettoken12345" → "mysec...2345"
	expected := "mysec...2345"
	if !bytes.Contains(out, []byte(expected)) {
		t.Errorf("expected redaction marker %q in output: %q", expected, out)
	}
}

func TestRedactBytesATATT(t *testing.T) {
	resetSecrets()
	// 40+ chars after "ATATT"
	atatt := "ATATTabcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJ"
	input := []byte("token is " + atatt + " end")
	out := RedactBytes(input)
	if bytes.Contains(out, []byte(atatt)) {
		t.Errorf("ATATT token still present in output: %q", out)
	}
	if !bytes.Contains(out, []byte("[REDACTED-ATATT]")) {
		t.Errorf("expected [REDACTED-ATATT] in output: %q", out)
	}
}

func TestRedactBytesUnregistered(t *testing.T) {
	resetSecrets()
	input := []byte("nothing sensitive here, just random content")
	out := RedactBytes(input)
	if string(out) != string(input) {
		t.Errorf("unrelated content was changed: got %q, want %q", out, input)
	}
}

func TestRegisterEmpty(t *testing.T) {
	resetSecrets()
	// Should be a no-op, no panic
	Register("")
	mu.RLock()
	n := len(secrets)
	mu.RUnlock()
	if n != 0 {
		t.Errorf("expected 0 registered secrets after Register(''), got %d", n)
	}
}

func TestRedactBytesShortToken(t *testing.T) {
	resetSecrets()
	// Token <= 9 chars — replaced with "xxxxx"
	Register("short")
	input := []byte("value: short here")
	out := RedactBytes(input)
	if strings.Contains(string(out), "short") {
		t.Errorf("short token still present: %q", out)
	}
	if !strings.Contains(string(out), "xxxxx") {
		t.Errorf("expected xxxxx for short token: %q", out)
	}
}
