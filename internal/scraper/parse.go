package scraper

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseWaitTime converts Ontario hospital wait time strings to total minutes.
// Assumed format based on provincial standard: "2 hours 30 minutes", "45 minutes", "1 hour".
// If a hospital uses a different format, add a hospital-specific parser in its own file.
func ParseWaitTime(text string) (int, error) {
	text = strings.ToLower(strings.TrimSpace(text))
	total := 0

	if idx := strings.Index(text, "hour"); idx != -1 {
		// text[:idx] slices everything before "hour", e.g. "2 hours 30 minutes" -> "2 "
		// lastWord("2 ") trims the space and returns "2"
		h, err := strconv.Atoi(lastWord(text[:idx]))
		if err != nil {
			return 0, fmt.Errorf("parse hours in %q: %w", text, err)
		}
		total += h * 60
	}
	if idx := strings.Index(text, "minute"); idx != -1 {
		// text[:idx] for "2 hours 30 minutes" -> "2 hours 30 ", lastWord returns "30"
		m, err := strconv.Atoi(lastWord(text[:idx]))
		if err != nil {
			return 0, fmt.Errorf("parse minutes in %q: %w", text, err)
		}
		total += m
	}
	if total == 0 {
		return 0, fmt.Errorf("unrecognised wait time format: %q", text)
	}
	return total, nil
}

// lastWord returns the last whitespace-separated token in s.
// lastWord("2 ") -> "2", lastWord("2 hours 30 ") -> "30"
func lastWord(s string) string {
	parts := strings.Fields(strings.TrimSpace(s))
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
