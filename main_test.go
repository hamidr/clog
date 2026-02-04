package main

import (
	"testing"
)

// --- truncate ---

func TestTruncate_WhenStringFitsWithinMax_ShouldReturnUnchanged(t *testing.T) {
	got := truncate("hello", 10)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestTruncate_WhenStringExceedsMax_ShouldTruncateWithEllipsis(t *testing.T) {
	got := truncate("hello world", 5)
	if got != "hello..." {
		t.Errorf("expected 'hello...', got %q", got)
	}
}

func TestTruncate_WhenStringExactlyMax_ShouldReturnUnchanged(t *testing.T) {
	got := truncate("12345", 5)
	if got != "12345" {
		t.Errorf("expected '12345', got %q", got)
	}
}

func TestTruncate_WhenEmptyString_ShouldReturnEmpty(t *testing.T) {
	got := truncate("", 10)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// --- formatToolInput ---

func TestFormatToolInput_WhenGivenBashCommand_ShouldExtractCommand(t *testing.T) {
	raw := `{"command":"ls -la","timeout":30}`
	got := formatToolInput("Bash", raw)
	if got != "ls -la" {
		t.Errorf("expected 'ls -la', got %q", got)
	}
}

func TestFormatToolInput_WhenGivenReadTool_ShouldExtractFilePath(t *testing.T) {
	raw := `{"file_path":"/tmp/test.go","offset":0}`
	got := formatToolInput("Read", raw)
	if got != "/tmp/test.go" {
		t.Errorf("expected '/tmp/test.go', got %q", got)
	}
}

func TestFormatToolInput_WhenGivenEditTool_ShouldExtractFilePath(t *testing.T) {
	raw := `{"file_path":"/tmp/main.go","old_string":"foo","new_string":"bar"}`
	got := formatToolInput("Edit", raw)
	if got != "/tmp/main.go" {
		t.Errorf("expected '/tmp/main.go', got %q", got)
	}
}

func TestFormatToolInput_WhenGivenWriteTool_ShouldExtractFilePath(t *testing.T) {
	raw := `{"file_path":"/tmp/output.txt","content":"data"}`
	got := formatToolInput("Write", raw)
	if got != "/tmp/output.txt" {
		t.Errorf("expected '/tmp/output.txt', got %q", got)
	}
}

func TestFormatToolInput_WhenGivenGlobTool_ShouldExtractPattern(t *testing.T) {
	raw := `{"pattern":"**/*.go"}`
	got := formatToolInput("Glob", raw)
	if got != "**/*.go" {
		t.Errorf("expected '**/*.go', got %q", got)
	}
}

func TestFormatToolInput_WhenGivenGrepTool_ShouldExtractPattern(t *testing.T) {
	raw := `{"pattern":"func main","path":"/tmp"}`
	got := formatToolInput("Grep", raw)
	if got != "func main" {
		t.Errorf("expected 'func main', got %q", got)
	}
}

func TestFormatToolInput_WhenGivenWebFetchTool_ShouldExtractURL(t *testing.T) {
	raw := `{"url":"https://example.com","prompt":"get title"}`
	got := formatToolInput("WebFetch", raw)
	if got != "https://example.com" {
		t.Errorf("expected 'https://example.com', got %q", got)
	}
}

func TestFormatToolInput_WhenGivenWebSearchTool_ShouldExtractQuery(t *testing.T) {
	raw := `{"query":"golang testing"}`
	got := formatToolInput("WebSearch", raw)
	if got != "golang testing" {
		t.Errorf("expected 'golang testing', got %q", got)
	}
}

func TestFormatToolInput_WhenGivenTaskTool_ShouldExtractPrompt(t *testing.T) {
	raw := `{"prompt":"find all errors","subagent_type":"Explore"}`
	got := formatToolInput("Task", raw)
	if got != "find all errors" {
		t.Errorf("expected 'find all errors', got %q", got)
	}
}

func TestFormatToolInput_WhenGivenUnknownTool_ShouldReturnTruncatedRawJSON(t *testing.T) {
	raw := `{"some_field":"some_value"}`
	got := formatToolInput("UnknownTool", raw)
	if got != raw {
		t.Errorf("expected raw JSON, got %q", got)
	}
}

func TestFormatToolInput_WhenGivenEmptyInput_ShouldReturnNoInputMessage(t *testing.T) {
	got := formatToolInput("Bash", "")
	if got != "(no input)" {
		t.Errorf("expected '(no input)', got %q", got)
	}
}

func TestFormatToolInput_WhenGivenInvalidJSON_ShouldReturnTruncatedRaw(t *testing.T) {
	raw := "this is not json"
	got := formatToolInput("Bash", raw)
	if got != raw {
		t.Errorf("expected raw fallback, got %q", got)
	}
}

func TestFormatToolInput_WhenCommandFieldIsMissing_ShouldFallbackToTruncatedRaw(t *testing.T) {
	raw := `{"timeout":30}`
	got := formatToolInput("Bash", raw)
	if got != raw {
		t.Errorf("expected raw fallback, got %q", got)
	}
}

func TestFormatToolInput_WhenInputExceedsTruncateLimit_ShouldTruncate(t *testing.T) {
	long := `{"command":"` + string(make([]byte, 200)) + `"}`
	got := formatToolInput("UnknownTool", long)
	if len(got) > 123 { // 120 + "..."
		t.Errorf("expected truncated output, got length %d", len(got))
	}
}

// --- fileExists ---

func TestFileExists_WhenFileExists_ShouldReturnTrue(t *testing.T) {
	// main.go always exists in the project root
	got := fileExists("main.go")
	if !got {
		t.Error("expected true for existing file")
	}
}

func TestFileExists_WhenFileDoesNotExist_ShouldReturnFalse(t *testing.T) {
	got := fileExists("/nonexistent/file/path.txt")
	if got {
		t.Error("expected false for non-existent file")
	}
}
