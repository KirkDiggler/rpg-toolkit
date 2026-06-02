package core

import "time"

// Clock is the source of game-event time the encounter stamps onto events at
// publish (North-Star Invariant 5: events carry game-event time, not
// wire-delivery time). Injecting it keeps the spine deterministic and
// testable — a test supplies a FixedClock so event timestamps are asserted
// exactly, and production uses the real wall clock.
type Clock interface {
	// Now returns the current game-event time.
	Now() time.Time
}

// SystemClock is the production Clock backed by the wall clock.
type SystemClock struct{}

// Now returns the current wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }

// FixedClock is a deterministic Clock that always returns the same instant.
// Tests use it so a published event's OccurredAt can be asserted exactly.
type FixedClock struct {
	// At is the instant Now returns.
	At time.Time
}

// Now returns the fixed instant.
func (c FixedClock) Now() time.Time { return c.At }

// compile-time checks that both clocks satisfy Clock.
var (
	_ Clock = SystemClock{}
	_ Clock = FixedClock{}
)
