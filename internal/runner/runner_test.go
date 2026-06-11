package runner

import (
	"testing"
	"time"
)

// nextInterval is the back-off function called every loop in runOne: given how
// many consecutive failures a Runnable has piled up, it decides how long to
// sleep before the next call. These tests pin down that decision.
func TestNextInterval(t *testing.T) {
	base := 15 * time.Minute

	tests := []struct {
		name  string
		fails int
		want  time.Duration
	}{
		{
			name: `
				given a Runnable that has never failed,
				when the scheduler picks the next sleep duration,
				then it uses the configured base interval`,
			fails: 0,
			want:  base,
		},
		{
			name: `
				given a Runnable that has failed once,
				when the scheduler picks the next sleep duration,
				then it still uses the base interval (one fail is not enough to back off)`,
			fails: 1,
			want:  base,
		},
		{
			name: `
				given a Runnable sitting right at the failure threshold minus one,
				when the scheduler picks the next sleep duration,
				then it still uses the base interval`,
			fails: BackoffThreshold - 1,
			want:  base,
		},
		{
			name: `
				given a Runnable that just crossed the failure threshold,
				when the scheduler picks the next sleep duration,
				then it doubles the base interval to slow down requests`,
			fails: BackoffThreshold,
			want:  2 * base,
		},
		{
			name: `
				given a Runnable one fail past the threshold,
				when the scheduler picks the next sleep duration,
				then it doubles again, landing on 4× base (which equals the MaxBackoff cap)`,
			fails: BackoffThreshold + 1,
			want:  4 * base,
		},
		{
			name: `
				given a Runnable two fails past the threshold,
				when the scheduler picks the next sleep duration,
				then it caps at MaxBackoff instead of doubling further`,
			fails: BackoffThreshold + 2,
			want:  MaxBackoff,
		},
		{
			name: `
				given a Runnable hammered by a long upstream outage (100 fails),
				when the scheduler picks the next sleep duration,
				then it stays clamped at MaxBackoff`,
			fails: 100,
			want:  MaxBackoff,
		},
		{
			name: `
				given an absurd fail count (1_000_000) far beyond any realistic outage,
				when the scheduler picks the next sleep duration,
				then the function still terminates and returns MaxBackoff (no int overflow)`,
			fails: 1_000_000,
			want:  MaxBackoff,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given: the failure count described in the table row.
			fails := tc.fails

			// When: the scheduler asks how long to sleep before the next call.
			got := nextInterval(base, fails)

			// Then: the returned duration matches the row's expected value.
			if got != tc.want {
				t.Fatalf("nextInterval(%v, %d) = %v, want %v", base, fails, got, tc.want)
			}
		})
	}
}

func TestNextInterval_NeverExceedsMaxAndNeverOverflows(t *testing.T) {
	t.Run(`
		given any plausible base interval combined with any fail count from 0 to 49,
		when the scheduler asks for the next sleep duration,
		then the returned duration always sits within (0, MaxBackoff] and never overflows int
		(this would catch a regression to 1<<shift arithmetic)`,
		func(t *testing.T) {
			// Given: a range of plausible base intervals and fail counts that span past the cap.
			bases := []time.Duration{1 * time.Second, 1 * time.Minute, 15 * time.Minute, 30 * time.Minute, 1 * time.Hour}

			for _, base := range bases {
				for fails := range 50 {
					// When: the scheduler computes the next sleep duration.
					got := nextInterval(base, fails)

					// Then: the result must stay inside the bounded range and never go negative.
					if got > MaxBackoff {
						t.Fatalf("nextInterval(%v, %d) = %v exceeds MaxBackoff %v", base, fails, got, MaxBackoff)
					}
					if got < 0 {
						t.Fatalf("nextInterval(%v, %d) = %v negative (overflow)", base, fails, got)
					}
				}
			}
		},
	)
}
