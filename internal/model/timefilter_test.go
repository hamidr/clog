package model

import (
	"testing"
	"time"
)

// --- parseRelativeDuration ---

func TestParseRelativeDuration_WhenGivenMinutes_ShouldReturnCorrectDuration(t *testing.T) {
	d, ok := parseRelativeDuration("30m")
	if !ok {
		t.Fatal("expected ok=true for '30m'")
	}
	if d != 30*time.Minute {
		t.Errorf("expected 30m, got %v", d)
	}
}

func TestParseRelativeDuration_WhenGivenHours_ShouldReturnCorrectDuration(t *testing.T) {
	d, ok := parseRelativeDuration("2h")
	if !ok {
		t.Fatal("expected ok=true for '2h'")
	}
	if d != 2*time.Hour {
		t.Errorf("expected 2h, got %v", d)
	}
}

func TestParseRelativeDuration_WhenGivenDays_ShouldReturnCorrectDuration(t *testing.T) {
	d, ok := parseRelativeDuration("7d")
	if !ok {
		t.Fatal("expected ok=true for '7d'")
	}
	if d != 7*24*time.Hour {
		t.Errorf("expected 7d, got %v", d)
	}
}

func TestParseRelativeDuration_WhenGivenWeeks_ShouldReturnCorrectDuration(t *testing.T) {
	d, ok := parseRelativeDuration("2w")
	if !ok {
		t.Fatal("expected ok=true for '2w'")
	}
	if d != 2*7*24*time.Hour {
		t.Errorf("expected 2w, got %v", d)
	}
}

func TestParseRelativeDuration_WhenGivenSingleCharacter_ShouldReturnFalse(t *testing.T) {
	_, ok := parseRelativeDuration("h")
	if ok {
		t.Error("expected ok=false for single-char input 'h'")
	}
}

func TestParseRelativeDuration_WhenGivenEmptyString_ShouldReturnFalse(t *testing.T) {
	_, ok := parseRelativeDuration("")
	if ok {
		t.Error("expected ok=false for empty string")
	}
}

func TestParseRelativeDuration_WhenGivenZeroValue_ShouldReturnFalse(t *testing.T) {
	_, ok := parseRelativeDuration("0h")
	if ok {
		t.Error("expected ok=false for zero duration")
	}
}

func TestParseRelativeDuration_WhenGivenNegativeValue_ShouldReturnFalse(t *testing.T) {
	_, ok := parseRelativeDuration("-3h")
	if ok {
		t.Error("expected ok=false for negative duration")
	}
}

func TestParseRelativeDuration_WhenGivenUnknownSuffix_ShouldReturnFalse(t *testing.T) {
	_, ok := parseRelativeDuration("5x")
	if ok {
		t.Error("expected ok=false for unknown suffix 'x'")
	}
}

func TestParseRelativeDuration_WhenGivenNonNumericPrefix_ShouldReturnFalse(t *testing.T) {
	_, ok := parseRelativeDuration("abch")
	if ok {
		t.Error("expected ok=false for non-numeric prefix")
	}
}

// --- parseTimeArg ---

func TestParseTimeArg_WhenGivenRelativeDuration_ShouldReturnTimeInThePast(t *testing.T) {
	before := time.Now().Add(-2 * time.Hour)
	result, err := parseTimeArg("2h")
	after := time.Now().Add(-2 * time.Hour)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Before(before.Add(-time.Second)) || result.After(after.Add(time.Second)) {
		t.Errorf("expected time ~2h ago, got %v", result)
	}
}

func TestParseTimeArg_WhenGivenDateOnly_ShouldParseCorrectly(t *testing.T) {
	result, err := parseTimeArg("2024-06-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestParseTimeArg_WhenGivenDateTimeWithoutSeconds_ShouldParseCorrectly(t *testing.T) {
	result, err := parseTimeArg("2024-06-15T14:30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestParseTimeArg_WhenGivenRFC3339_ShouldParseCorrectly(t *testing.T) {
	result, err := parseTimeArg("2024-06-15T14:30:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestParseTimeArg_WhenGivenRFC3339WithOffset_ShouldParseCorrectly(t *testing.T) {
	result, err := parseTimeArg("2024-06-15T14:30:00+05:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	loc := time.FixedZone("", 5*3600)
	expected := time.Date(2024, 6, 15, 14, 30, 0, 0, loc)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestParseTimeArg_WhenGivenGarbage_ShouldReturnError(t *testing.T) {
	_, err := parseTimeArg("garbage")
	if err == nil {
		t.Error("expected error for garbage input")
	}
}

func TestParseTimeArg_WhenGivenPartialDate_ShouldReturnError(t *testing.T) {
	_, err := parseTimeArg("2024-06")
	if err == nil {
		t.Error("expected error for partial date")
	}
}

// --- ParseTimeFilter ---

func TestParseTimeFilter_WhenBothEmpty_ShouldReturnNil(t *testing.T) {
	tf, err := ParseTimeFilter("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tf != nil {
		t.Error("expected nil TimeFilter when both args are empty")
	}
}

func TestParseTimeFilter_WhenOnlySinceProvided_ShouldSetSinceAndLeaveUntilNil(t *testing.T) {
	tf, err := ParseTimeFilter("1d", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tf == nil {
		t.Fatal("expected non-nil TimeFilter")
	}
	if tf.Since == nil {
		t.Error("expected Since to be set")
	}
	if tf.Until != nil {
		t.Error("expected Until to be nil")
	}
}

func TestParseTimeFilter_WhenOnlyUntilProvided_ShouldSetUntilAndLeaveSinceNil(t *testing.T) {
	tf, err := ParseTimeFilter("", "2024-12-31")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tf == nil {
		t.Fatal("expected non-nil TimeFilter")
	}
	if tf.Since != nil {
		t.Error("expected Since to be nil")
	}
	if tf.Until == nil {
		t.Error("expected Until to be set")
	}
	expected := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	if !tf.Until.Equal(expected) {
		t.Errorf("expected Until=%v, got %v", expected, *tf.Until)
	}
}

func TestParseTimeFilter_WhenBothProvided_ShouldSetBoth(t *testing.T) {
	tf, err := ParseTimeFilter("2024-01-01", "2024-12-31")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tf == nil {
		t.Fatal("expected non-nil TimeFilter")
	}
	if tf.Since == nil {
		t.Error("expected Since to be set")
	}
	if tf.Until == nil {
		t.Error("expected Until to be set")
	}
}

func TestParseTimeFilter_WhenSinceIsInvalid_ShouldReturnError(t *testing.T) {
	_, err := ParseTimeFilter("not-a-time", "")
	if err == nil {
		t.Fatal("expected error for invalid --since")
	}
	if got := err.Error(); !contains(got, "--since") {
		t.Errorf("expected error to mention --since, got: %s", got)
	}
}

func TestParseTimeFilter_WhenUntilIsInvalid_ShouldReturnError(t *testing.T) {
	_, err := ParseTimeFilter("", "not-a-time")
	if err == nil {
		t.Fatal("expected error for invalid --until")
	}
	if got := err.Error(); !contains(got, "--until") {
		t.Errorf("expected error to mention --until, got: %s", got)
	}
}

func TestParseTimeFilter_WhenSinceIsRelativeAndUntilIsAbsolute_ShouldParseBoth(t *testing.T) {
	tf, err := ParseTimeFilter("2h", "2024-12-31")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tf.Since == nil || tf.Until == nil {
		t.Fatal("expected both Since and Until to be set")
	}
	// Since should be approximately 2 hours ago
	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	diff := tf.Since.Sub(twoHoursAgo)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("expected Since ~2h ago, got %v (diff=%v)", *tf.Since, diff)
	}
	// Until should be 2024-12-31
	expectedUntil := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	if !tf.Until.Equal(expectedUntil) {
		t.Errorf("expected Until=%v, got %v", expectedUntil, *tf.Until)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
