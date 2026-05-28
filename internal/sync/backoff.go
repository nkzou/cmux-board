package sync

import "time"

// Backoff implements a capped exponential backoff sequence for the poll error path.
// Sequence: 60s → 2m → 5m (cap). Reset() restarts the sequence from 60s.
// Zero value is ready to use.
type Backoff struct {
	step int // 0-indexed; clamped to len(backoffSteps)-1
}

// Named constants — no magic numbers.
const (
	backoffBase = 60 * time.Second
	backoffCap  = 5 * time.Minute
)

var backoffSteps = []time.Duration{
	60 * time.Second,
	2 * time.Minute,
	5 * time.Minute,
}

// Next returns the next backoff duration and advances the internal step counter.
// Once the cap (5m) is reached, every subsequent call returns the cap.
func (b *Backoff) Next() time.Duration {
	d := backoffSteps[b.step]
	if b.step < len(backoffSteps)-1 {
		b.step++
	}
	return d
}

// Reset restarts the sequence from 60s.
func (b *Backoff) Reset() {
	b.step = 0
}
