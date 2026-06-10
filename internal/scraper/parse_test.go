package scraper

import "testing"

// ParseWaitTime takes the raw text scraped off a hospital page (e.g. "2 hours 30 minutes")
// and converts it to a total minute count. Success and failure paths assert against very
// different things, so each gets its own test.

func TestParseWaitTime_Success(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{
			name: `
				given a hospital page that publishes wait as '2 hours 30 minutes',
				when the scraper passes it to ParseWaitTime,
				then it returns 150 total minutes`,
			in:   "2 hours 30 minutes",
			want: 150,
		},
		{
			name: `
				given a hospital page that publishes wait as '1 hour' (no minutes),
				when ParseWaitTime parses it,
				then it returns 60 minutes`,
			in:   "1 hour",
			want: 60,
		},
		{
			name: `
				given a hospital page using the plural 'hours' only ('3 hours'),
				when ParseWaitTime parses it,
				then it returns 180 minutes`,
			in:   "3 hours",
			want: 180,
		},
		{
			name: `
				given a hospital page reporting minutes only ('45 minutes'),
				when ParseWaitTime parses it,
				then it returns the raw minute count`,
			in:   "45 minutes",
			want: 45,
		},
		{
			name: `
				given a hospital page reporting the singular '1 minute',
				when ParseWaitTime parses it,
				then it returns 1`,
			in:   "1 minute",
			want: 1,
		},
		{
			name: `
				given a hospital page that uppercases the unit words ('2 HOURS 15 MINUTES'),
				when ParseWaitTime parses it,
				then it normalises case and returns 135`,
			in:   "2 HOURS 15 MINUTES",
			want: 135,
		},
		{
			name: `
				given a hospital page wrapping the value in extra whitespace,
				when ParseWaitTime parses it,
				then it trims and returns the right minute total`,
			in:   "   1 hour 5 minutes   ",
			want: 65,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given: the raw wait-time text from the table row.
			input := tc.in

			// When: the scraper asks ParseWaitTime to convert that text to minutes.
			got, err := ParseWaitTime(input)

			// Then: no error, and the minute total matches.
			if err != nil {
				t.Fatalf("ParseWaitTime(%q) unexpected error: %v", input, err)
			}
			if got != tc.want {
				t.Fatalf("ParseWaitTime(%q) = %d, want %d", input, got, tc.want)
			}
		})
	}
}

func TestParseWaitTime_Failure(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{
			name: `
				given an empty string (page returned nothing recognisable),
				when ParseWaitTime is called,
				then it returns an error and the orchestrator marks the scrape failed`,
			in: "",
		},
		{
			name: `
				given an unrecognised word like 'soon',
				when ParseWaitTime is called,
				then it returns an error`,
			in: "soon",
		},
		{
			name: `
				given hour text with a non-numeric token ('abc hours'),
				when ParseWaitTime is called,
				then it returns an error and refuses to produce a bogus minute count`,
			in: "abc hours",
		},
		{
			name: `
				given minute text with a non-numeric token ('xy minutes'),
				when ParseWaitTime is called,
				then it returns an error`,
			in: "xy minutes",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given: malformed text from the table row.
			input := tc.in

			// When: the scraper asks ParseWaitTime to convert it.
			_, err := ParseWaitTime(input)

			// Then: the parser refuses with a non-nil error.
			if err == nil {
				t.Fatalf("ParseWaitTime(%q) returned no error, want error", input)
			}
		})
	}
}

// lastWord is the helper ParseWaitTime uses to grab "30" out of "2 hours 30 ".
func TestLastWord(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: `
				given a single token plus trailing whitespace,
				when lastWord is called,
				then it returns just the token without the whitespace`,
			in:   "2 ",
			want: "2",
		},
		{
			name: `
				given multiple tokens with trailing whitespace,
				when lastWord is called,
				then it returns the last token only`,
			in:   "2 hours 30 ",
			want: "30",
		},
		{
			name: `
				given an empty string,
				when lastWord is called,
				then it returns empty`,
			in:   "",
			want: "",
		},
		{
			name: `
				given a whitespace-only string,
				when lastWord is called,
				then it returns empty`,
			in:   "   ",
			want: "",
		},
		{
			name: `
				given a single word with no whitespace,
				when lastWord is called,
				then it returns that word`,
			in:   "only",
			want: "only",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given: the raw string from the table row.
			input := tc.in

			// When: the parser asks lastWord to return the trailing token.
			got := lastWord(input)

			// Then: it matches the expected token.
			if got != tc.want {
				t.Errorf("lastWord(%q) = %q, want %q", input, got, tc.want)
			}
		})
	}
}
