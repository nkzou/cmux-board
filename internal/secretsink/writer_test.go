package secretsink

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriterRedacts(t *testing.T) {
	resetSecrets()
	token := "supersecretapitoken99"
	Register(token)
	var buf bytes.Buffer
	w := Writer(&buf)
	input := []byte("calling api with token: " + token + " done")
	n, err := w.Write(input)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	_ = n
	if strings.Contains(buf.String(), token) {
		t.Errorf("token still in writer output: %q", buf.String())
	}
	// For "supersecretapitoken99" (21 chars > 9): "super...en99"
	expected := "super...en99"
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected redaction marker %q in writer output: %q", expected, buf.String())
	}
}

func TestWriterLengthReturn(t *testing.T) {
	resetSecrets()
	token := "longerthanninecharstoken"
	Register(token)
	var buf bytes.Buffer
	w := Writer(&buf)
	input := []byte("secret is " + token + " here")
	n, err := w.Write(input)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Must return original length, not redacted length
	if n != len(input) {
		t.Errorf("Write returned n=%d, want len(input)=%d", n, len(input))
	}
}
