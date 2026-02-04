package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- extractText ---

func TestExtractText_WhenGivenPlainString_ShouldReturnItDirectly(t *testing.T) {
	raw := json.RawMessage(`"hello world"`)
	got := extractText(raw)
	if got != "hello world" {
		t.Errorf("expected 'hello world', got %q", got)
	}
}

func TestExtractText_WhenGivenArrayOfTextBlocks_ShouldJoinWithNewlines(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"line one"},{"type":"text","text":"line two"}]`)
	got := extractText(raw)
	expected := "line one\nline two"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestExtractText_WhenGivenArrayWithNonTextBlocks_ShouldSkipThem(t *testing.T) {
	raw := json.RawMessage(`[{"type":"tool_use","id":"tu-1"},{"type":"text","text":"the answer"}]`)
	got := extractText(raw)
	if got != "the answer" {
		t.Errorf("expected 'the answer', got %q", got)
	}
}

func TestExtractText_WhenGivenEmptyArray_ShouldReturnEmptyString(t *testing.T) {
	raw := json.RawMessage(`[]`)
	got := extractText(raw)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractText_WhenGivenEmptyRawMessage_ShouldReturnEmptyString(t *testing.T) {
	got := extractText(json.RawMessage{})
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractText_WhenGivenNilRawMessage_ShouldReturnEmptyString(t *testing.T) {
	got := extractText(nil)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractText_WhenGivenTextBlockWithEmptyText_ShouldSkipIt(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":""},{"type":"text","text":"visible"}]`)
	got := extractText(raw)
	if got != "visible" {
		t.Errorf("expected 'visible', got %q", got)
	}
}

func TestExtractText_WhenGivenInvalidJSON_ShouldReturnRawString(t *testing.T) {
	raw := json.RawMessage(`not valid json`)
	got := extractText(raw)
	if got != "not valid json" {
		t.Errorf("expected raw fallback, got %q", got)
	}
}

// --- parseTimestamp ---

func TestParseTimestamp_WhenGivenValidRFC3339Nano_ShouldParseCorrectly(t *testing.T) {
	input := "2024-06-15T14:30:00.123456789Z"
	fallback := time.Now()
	got := parseTimestamp(input, fallback)

	expected := time.Date(2024, 6, 15, 14, 30, 0, 123456789, time.UTC)
	if !got.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestParseTimestamp_WhenGivenEmptyString_ShouldReturnFallback(t *testing.T) {
	fallback := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	got := parseTimestamp("", fallback)
	if !got.Equal(fallback) {
		t.Errorf("expected fallback %v, got %v", fallback, got)
	}
}

func TestParseTimestamp_WhenGivenInvalidFormat_ShouldReturnFallback(t *testing.T) {
	fallback := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	got := parseTimestamp("not-a-timestamp", fallback)
	if !got.Equal(fallback) {
		t.Errorf("expected fallback %v, got %v", fallback, got)
	}
}

func TestParseTimestamp_WhenGivenRFC3339WithoutNanos_ShouldParseCorrectly(t *testing.T) {
	fallback := time.Now()
	got := parseTimestamp("2024-06-15T14:30:00Z", fallback)
	expected := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
	if !got.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

// --- Harvest ---

func writeTranscript(t *testing.T, dir string, lines ...string) string {
	t.Helper()
	path := filepath.Join(dir, "transcript.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, l := range lines {
		f.WriteString(l + "\n")
	}
	return path
}

func TestHarvest_WhenGivenUserMessage_ShouldExtractIt(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir,
		`{"type":"message","uuid":"u1","message":{"role":"user","content":"hello"}}`,
	)

	result, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("expected role 'user', got %q", result.Messages[0].Role)
	}
	if result.Messages[0].Content != "hello" {
		t.Errorf("expected content 'hello', got %q", result.Messages[0].Content)
	}
	if result.Messages[0].SessionID != "sess-1" {
		t.Errorf("expected session ID 'sess-1', got %q", result.Messages[0].SessionID)
	}
}

func TestHarvest_WhenGivenAssistantMessage_ShouldExtractTextBlocks(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir,
		`{"type":"message","uuid":"a1","message":{"role":"assistant","content":[{"type":"text","text":"answer"}],"model":"claude-3"}}`,
	)

	result, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
	if result.Messages[0].Content != "answer" {
		t.Errorf("expected content 'answer', got %q", result.Messages[0].Content)
	}
	if result.Messages[0].Model != "claude-3" {
		t.Errorf("expected model 'claude-3', got %q", result.Messages[0].Model)
	}
}

func TestHarvest_WhenGivenSystemMessage_ShouldSkipIt(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir,
		`{"type":"message","uuid":"s1","message":{"role":"system","content":"you are helpful"}}`,
		`{"type":"message","uuid":"u1","message":{"role":"user","content":"hi"}}`,
	)

	result, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message (system skipped), got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("expected role 'user', got %q", result.Messages[0].Role)
	}
}

func TestHarvest_WhenGivenLinesWithoutMessage_ShouldSkipThem(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir,
		`{"type":"summary","content":"this is a summary"}`,
		`{"type":"message","uuid":"u1","message":{"role":"user","content":"hi"}}`,
	)

	result, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
}

func TestHarvest_WhenGivenEmptyFile_ShouldReturnNoMessages(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir)

	result, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(result.Messages))
	}
}

func TestHarvest_WhenGivenInvalidJSONLines_ShouldSkipThemGracefully(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir,
		`not json at all`,
		`{"type":"message","uuid":"u1","message":{"role":"user","content":"valid"}}`,
	)

	result, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message (invalid line skipped), got %d", len(result.Messages))
	}
}

func TestHarvest_WhenGivenEmptyLines_ShouldSkipThem(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir,
		``,
		`{"type":"message","uuid":"u1","message":{"role":"user","content":"hi"}}`,
		``,
	)

	result, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
}

func TestHarvest_ShouldReturnNewOffsetGreaterThanZero(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir,
		`{"type":"message","uuid":"u1","message":{"role":"user","content":"hello"}}`,
	)

	result, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NewOffset <= 0 {
		t.Errorf("expected NewOffset > 0, got %d", result.NewOffset)
	}
}

func TestHarvest_WhenResumedFromOffset_ShouldOnlyReadNewLines(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir,
		`{"type":"message","uuid":"u1","message":{"role":"user","content":"first"}}`,
		`{"type":"message","uuid":"u2","message":{"role":"user","content":"second"}}`,
	)

	// First harvest
	result1, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result1.Messages) != 2 {
		t.Fatalf("expected 2 messages on first harvest, got %d", len(result1.Messages))
	}

	// Second harvest from the saved offset — should find nothing new
	result2, err := Harvest("sess-1", path, result1.NewOffset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result2.Messages) != 0 {
		t.Errorf("expected 0 new messages, got %d", len(result2.Messages))
	}
}

func TestHarvest_WhenResumedAndNewDataAppended_ShouldReturnOnlyNewMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")

	// Write first line
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"type":"message","uuid":"u1","message":{"role":"user","content":"first"}}` + "\n")
	f.Close()

	// First harvest
	result1, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result1.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result1.Messages))
	}

	// Append second line
	f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"type":"message","uuid":"u2","message":{"role":"user","content":"second"}}` + "\n")
	f.Close()

	// Second harvest from offset — should only get the new message
	result2, err := Harvest("sess-1", path, result1.NewOffset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result2.Messages) != 1 {
		t.Fatalf("expected 1 new message, got %d", len(result2.Messages))
	}
	if result2.Messages[0].Content != "second" {
		t.Errorf("expected 'second', got %q", result2.Messages[0].Content)
	}
}

func TestHarvest_ShouldPreserveUUIDAndParentUUID(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir,
		`{"type":"message","uuid":"msg-1","parentUuid":"parent-1","message":{"role":"user","content":"hi"}}`,
	)

	result, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Messages[0].UUID != "msg-1" {
		t.Errorf("expected UUID 'msg-1', got %q", result.Messages[0].UUID)
	}
	if result.Messages[0].ParentUUID != "parent-1" {
		t.Errorf("expected ParentUUID 'parent-1', got %q", result.Messages[0].ParentUUID)
	}
}

func TestHarvest_WhenGivenTimestamp_ShouldParseIt(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir,
		`{"type":"message","uuid":"u1","timestamp":"2024-06-15T14:30:00Z","message":{"role":"user","content":"hi"}}`,
	)

	result, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
	if !result.Messages[0].Timestamp.Equal(expected) {
		t.Errorf("expected timestamp %v, got %v", expected, result.Messages[0].Timestamp)
	}
}

func TestHarvest_WhenTimestampMissing_ShouldUseFallbackTime(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir,
		`{"type":"message","uuid":"u1","message":{"role":"user","content":"hi"}}`,
	)

	before := time.Now().UTC()
	result, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	after := time.Now().UTC()

	ts := result.Messages[0].Timestamp
	if ts.Before(before.Add(-time.Second)) || ts.After(after.Add(time.Second)) {
		t.Errorf("expected timestamp near now, got %v", ts)
	}
}

func TestHarvest_WhenFileDoesNotExist_ShouldReturnError(t *testing.T) {
	_, err := Harvest("sess-1", "/nonexistent/path/transcript.jsonl", 0)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestHarvest_ShouldPreserveRawContent(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir,
		`{"type":"message","uuid":"u1","message":{"role":"user","content":"raw text"}}`,
	)

	result, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Messages[0].RawContent != `"raw text"` {
		t.Errorf("expected raw content '\"raw text\"', got %q", result.Messages[0].RawContent)
	}
}

func TestHarvest_WhenGivenMultipleMessages_ShouldReturnAllInOrder(t *testing.T) {
	dir := t.TempDir()
	path := writeTranscript(t, dir,
		`{"type":"message","uuid":"u1","message":{"role":"user","content":"one"}}`,
		`{"type":"message","uuid":"a1","message":{"role":"assistant","content":[{"type":"text","text":"two"}]}}`,
		`{"type":"message","uuid":"u2","message":{"role":"user","content":"three"}}`,
	)

	result, err := Harvest("sess-1", path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result.Messages))
	}
	contents := []string{result.Messages[0].Content, result.Messages[1].Content, result.Messages[2].Content}
	expected := []string{"one", "two", "three"}
	for i, c := range contents {
		if c != expected[i] {
			t.Errorf("message[%d]: expected %q, got %q", i, expected[i], c)
		}
	}
}
