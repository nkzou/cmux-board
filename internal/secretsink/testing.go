//go:build testing

package secretsink

// RegisteredCount returns the number of secrets currently registered.
// This helper is exported only under the "testing" build tag and is intended
// for use in tests that verify secretsink.Register was called correctly.
func RegisteredCount() int {
	mu.RLock()
	defer mu.RUnlock()
	return len(secrets)
}
