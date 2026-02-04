package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TimeFilter holds optional time bounds for search queries.
type TimeFilter struct {
	Since *time.Time
	Until *time.Time
}

// ParseTimeFilter parses since/until strings into a TimeFilter.
// Returns nil if both are empty.
func ParseTimeFilter(sinceStr, untilStr string) (*TimeFilter, error) {
	if sinceStr == "" && untilStr == "" {
		return nil, nil
	}

	tf := &TimeFilter{}

	if sinceStr != "" {
		t, err := parseTimeArg(sinceStr)
		if err != nil {
			return nil, fmt.Errorf("invalid --since value %q: %w", sinceStr, err)
		}
		tf.Since = &t
	}

	if untilStr != "" {
		t, err := parseTimeArg(untilStr)
		if err != nil {
			return nil, fmt.Errorf("invalid --until value %q: %w", untilStr, err)
		}
		tf.Until = &t
	}

	return tf, nil
}

// parseTimeArg tries to parse a time argument as a relative duration (e.g. "2h", "1d"),
// then falls back to absolute timestamp formats.
func parseTimeArg(s string) (time.Time, error) {
	if d, ok := parseRelativeDuration(s); ok {
		return time.Now().Add(-d), nil
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02",
	}

	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("expected relative duration (30m, 2h, 1d, 1w) or timestamp (2006-01-02, 2006-01-02T15:04, RFC3339)")
}

// parseRelativeDuration handles suffixes: m (minutes), h (hours), d (days), w (weeks).
func parseRelativeDuration(s string) (time.Duration, bool) {
	if len(s) < 2 {
		return 0, false
	}

	suffix := s[len(s)-1]
	numStr := strings.TrimSpace(s[:len(s)-1])

	n, err := strconv.Atoi(numStr)
	if err != nil || n <= 0 {
		return 0, false
	}

	switch suffix {
	case 'm':
		return time.Duration(n) * time.Minute, true
	case 'h':
		return time.Duration(n) * time.Hour, true
	case 'd':
		return time.Duration(n) * 24 * time.Hour, true
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, true
	default:
		return 0, false
	}
}
