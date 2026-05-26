package secretsink

import (
	"fmt"
	"testing"
)

func BenchmarkRedactBytes10Secrets(b *testing.B) {
	resetSecrets()
	// Register 10 distinct secrets
	for i := range 10 {
		Register(fmt.Sprintf("supersecrettoken%04d", i))
	}
	line := []byte(`{"level":"INFO","time":"2026-05-26T10:00:00Z","msg":"fetching board","user":"user@example.com","status":200}`)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = RedactBytes(line)
	}
}
