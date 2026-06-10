// Package clock is the wall-time seam shared by code that timestamps records
// (scrapers, collectors).
package clock

import "time"

// Clock returns the current instant. All implementations must be safe for
// concurrent use because production callers run inside goroutines.
type Clock interface {
	Now() time.Time
}

// Ensure RealClock satisfies Clock at compile time.
var _ Clock = RealClock{}

// RealClock returns UTC wall time on every call. UTC, not local, so persisted
// timestamps compare cleanly across hosts.
type RealClock struct{}

// Now satisfies Clock.
func (RealClock) Now() time.Time { return time.Now().UTC() }
