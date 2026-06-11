// Package runner is the periodic scheduler that drives every Runnable in the
// app. Each Runnable runs in its own goroutine, with startup jitter, per-call
// timeout, exponential backoff on consecutive failures, and ctx-aware sleep.
// One contract (Runnable) covers both wait-time scrapers and external-signal
// collectors so the loop lives in exactly one place.
package runner

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"
)

const (
	// MaxBackoff caps the sleep duration applied after consecutive failures.
	MaxBackoff = 1 * time.Hour
	// BackoffThreshold is the number of consecutive failures tolerated before
	// the runner starts doubling the sleep duration.
	BackoffThreshold = 3
	// Timeout bounds a single Runnable.Run call. The runner cancels the call
	// context after this duration so a hung upstream cannot stall the loop.
	Timeout = 30 * time.Second
)

// ErrSkip lets a Runnable signal "not a real failure, not a success either":
// leave the backoff counter untouched. Used for upstream-reported transient
// absence states (e.g. Montfort's "--") where the source is reachable but
// reports no data, and a doubled sleep would only delay catching recovery.
var ErrSkip = errors.New("runner: skip (no fail, no reset)")

// Runnable is the only contract the scheduler needs. Both wait-time scrapers
// and external-signal collectors implement this directly: no per-domain
// interface, no adapter.
type Runnable interface {
	// Name is the stable identifier used in scheduler logs.
	Name() string
	// Interval is the steady-state wait between successful calls.
	Interval() time.Duration
	// Run performs one unit of work. Returning ErrSkip leaves the failure
	// counter untouched; any other non-nil error bumps it and feeds backoff.
	Run(ctx context.Context) error
}

// Base provides the Name/Interval boilerplate for Runnables that need nothing
// else from the scheduler. Domain-specific helpers (e.g. scraper.BaseScraper)
// embed Base alongside their own state.
type Base struct {
	name  string
	every time.Duration
}

// NewBase returns a Base carrying the given Runnable identity and interval.
func NewBase(name string, every time.Duration) Base {
	return Base{name: name, every: every}
}

func (b Base) Name() string            { return b.name }
func (b Base) Interval() time.Duration { return b.every }

// Run starts one goroutine per Runnable and blocks until ctx is cancelled.
// SIGINT/SIGTERM should arrive via signal.NotifyContext at the caller.
func Run[T Runnable](ctx context.Context, items []T, log *slog.Logger) {
	var wg sync.WaitGroup
	for _, it := range items {
		wg.Add(1)
		go func(it T) {
			defer wg.Done()
			runOne(ctx, it, log)
		}(it)
	}
	wg.Wait() // returns once every goroutine has exited via ctx cancellation
}

// runOne owns the per-item loop. Each iteration: jitter (first only) →
// WithTimeout → item.Run → classify outcome → ctx-aware sleep.
func runOne[T Runnable](ctx context.Context, item T, log *slog.Logger) {
	// Stagger first call so a fleet of Runnables sharing the scheduler does
	// not hit every upstream at the same wall-clock moment after boot.
	jitter := time.Duration(rand.N(int64(30 * time.Second))) //nolint:gosec // G404: stagger jitter, not crypto
	select {
	case <-ctx.Done(): // shutdown before first run; exit immediately
		return
	case <-time.After(jitter):
	}

	consecutiveFails := 0

	for {
		callCtx, cancel := context.WithTimeout(ctx, Timeout)
		err := item.Run(callCtx)
		cancel() // release the WithTimeout timer; not calling it leaks a goroutine per loop

		switch {
		case err == nil:
			consecutiveFails = 0
		case errors.Is(err, ErrSkip):
			// closure already logged the warn (transient state); preserve
			// backoff progression from any prior real failures
		default:
			consecutiveFails++
			log.Error("step failed", "name", item.Name(), "err", err, "consecutive_fails", consecutiveFails)
		}

		next := nextInterval(item.Interval(), consecutiveFails)
		// time.Duration renders as int64 nanoseconds under slog's JSON handler.
		// Stringify so the log shows e.g. "30m0s" instead of 1800000000000.
		log.Debug("step sleeping", "name", item.Name(), "next_in", next.String())

		select {
		case <-ctx.Done(): // SIGINT/SIGTERM received; exit cleanly instead of sleeping
			return
		case <-time.After(next):
			// sleep elapsed; loop back to the next Run call. ctx.Done() is
			// checked above so a cancel during the sleep wins the race.
		}
	}
}

// nextInterval returns the wait duration before the next call.
// Normal: use the Runnable's configured interval.
// After BackoffThreshold consecutive failures: double each time, capped at MaxBackoff.
// Doubles step-by-step with a min() cap each step so the multiplier can never overflow.
func nextInterval(base time.Duration, fails int) time.Duration {
	if fails < BackoffThreshold {
		return base
	}
	next := base
	for range fails - BackoffThreshold + 1 {
		next = min(next*2, MaxBackoff)
	}
	return next
}
