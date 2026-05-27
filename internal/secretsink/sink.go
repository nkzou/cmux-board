package secretsink

import (
	"bytes"
	"regexp"
	"sync"
)

var (
	mu      sync.RWMutex
	secrets [][]byte
	atattRe = regexp.MustCompile(`ATATT[A-Za-z0-9_\-]{40,}`)
)

// Register adds a raw secret value to the redaction set.
// Only the raw API token should be registered (NOT email, NOT base64 Basic value).
// Register is idempotent: duplicate values are skipped.
func Register(value string) {
	if value == "" {
		return
	}
	b := []byte(value)
	mu.Lock()
	defer mu.Unlock()
	for _, s := range secrets {
		if string(s) == value {
			return // already registered
		}
	}
	secrets = append(secrets, b)
}

// IsRegistered reports whether value is in the redaction set.
// Used in tests to verify ordering: secretsink.Register before NewLogger.
func IsRegistered(value string) bool {
	if value == "" {
		return false
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, s := range secrets {
		if string(s) == value {
			return true
		}
	}
	return false
}

// RedactBytes scans b for any registered secret and replaces each occurrence
// with "xxxxx...xxxx" (first 5 + last 4 chars of the registered value).
// Also applies the ATATT regex backstop as a catchall.
func RedactBytes(b []byte) []byte {
	mu.RLock()
	localSecrets := make([][]byte, len(secrets))
	copy(localSecrets, secrets)
	mu.RUnlock()

	for _, s := range localSecrets {
		if len(s) == 0 {
			continue
		}
		var replacement []byte
		if len(s) > 9 {
			replacement = append(s[:5:5], append([]byte("..."), s[len(s)-4:]...)...)
		} else {
			replacement = []byte("xxxxx")
		}
		b = bytes.ReplaceAll(b, s, replacement)
	}
	// ATATT backstop
	b = atattRe.ReplaceAll(b, []byte("[REDACTED-ATATT]"))
	return b
}
