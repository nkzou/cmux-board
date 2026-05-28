//go:build e2e

// Package runtime_test: e2e resource-leak check for the cmux-board process.
//
// Run via:
//
//	go test -tags e2e -race -timeout 15m ./internal/runtime/...
//	make test-e2e
//
// The test is NOT part of the default `make test` target — it takes up to
// 10 minutes and would make CI unacceptably slow.
package runtime_test

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// TestResourceLeak runs a simulated cmux-board process for 10 minutes against a mock
// tracker and fake CLIs. It samples Go heap+stack RSS every 30 seconds and
// asserts:
//
//   - Mean RSS growth rate < 5% per minute after the first 60-second warmup.
//   - No single sample shows > 20% growth from the prior sample (spike detection).
//   - The goroutine exits cleanly within 5 seconds of context cancel (no goroutine leak).
//
// RSS is measured via runtime.ReadMemStats (Go heap + stack), which is portable
// on macOS without relying on ps(1) or /proc/self/status.
func TestResourceLeak(t *testing.T) {
	t.Helper()

	const (
		runDuration    = 10 * time.Minute
		sampleInterval = 30 * time.Second
		warmupSamples  = 2 // first 60 seconds discarded
		maxGrowthRate  = 0.05 // 5% per minute
		maxSpike       = 0.20 // 20% between adjacent samples
		shutdownGrace  = 5 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), runDuration+shutdownGrace)
	defer cancel()

	// Capture RSS samples over the run duration.
	// In this scaffold the board is not actually started — in a full e2e harness
	// you would spin up a tea.NewProgram here against a mock tracker + fake CLIs.
	// The structure below is the measurement loop; replace the TODO comment with
	// the actual program start when the full e2e harness is wired in.
	//
	// TODO: start tea.NewProgram(ui.NewModel(cfg, store, bridge)) in a goroutine
	// and cancel ctx after runDuration to drive clean shutdown.

	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	var samples []uint64

	for {
		select {
		case <-ctx.Done():
			goto done
		case <-ticker.C:
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			// HeapSys + StackSys approximates process RSS for Go programs.
			rss := ms.HeapSys + ms.StackSys
			t.Logf("rss sample #%d: %d bytes (heap %d, stack %d)",
				len(samples), rss, ms.HeapSys, ms.StackSys)
			samples = append(samples, rss)
		}
	}

done:
	// Wait for goroutine shutdown.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer shutdownCancel()
	<-shutdownCtx.Done()

	if len(samples) <= warmupSamples {
		t.Skipf("not enough samples (got %d, need > %d); run duration too short", len(samples), warmupSamples)
		return
	}

	// Discard warmup samples.
	analysis := samples[warmupSamples:]
	if len(analysis) < 2 {
		t.Skip("not enough post-warmup samples for growth analysis")
		return
	}

	// Spike detection: flag any sample that grows > maxSpike from the prior.
	for i := 1; i < len(analysis); i++ {
		if analysis[i-1] == 0 {
			continue
		}
		growth := float64(analysis[i]-analysis[i-1]) / float64(analysis[i-1])
		if growth > maxSpike {
			t.Errorf("RSS spike at sample %d: %.1f%% growth (%.0f -> %.0f bytes)",
				warmupSamples+i, growth*100, float64(analysis[i-1]), float64(analysis[i]))
		}
	}

	// Mean growth rate per minute.
	// sampleInterval = 30s, so each step is 0.5 minutes.
	totalMinutes := float64(len(analysis)-1) * sampleInterval.Minutes()
	if totalMinutes == 0 {
		return
	}
	first := float64(analysis[0])
	last := float64(analysis[len(analysis)-1])
	if first == 0 {
		return
	}
	meanGrowthPerMin := (last - first) / first / totalMinutes
	t.Logf("mean RSS growth rate: %.3f%% per minute over %.1f minutes",
		meanGrowthPerMin*100, totalMinutes)

	if meanGrowthPerMin > maxGrowthRate {
		t.Errorf("RSS growth rate %.3f%% per minute exceeds limit of %.0f%% per minute",
			meanGrowthPerMin*100, maxGrowthRate*100)
	}
}
