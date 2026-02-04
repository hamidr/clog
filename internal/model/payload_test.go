package model

import (
	"encoding/json"
	"testing"
)

// --- ParsePayload: valid inputs ---

func TestParsePayload_WhenGivenMinimalValidJSON_ShouldReturnParsedPayload(t *testing.T) {
	input := `{"session_id":"sess-1","cwd":"/tmp","hook_event_name":"Stop"}`
	got, err := ParsePayload([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Session.ID != "sess-1" {
		t.Errorf("expected session ID 'sess-1', got %q", got.Session.ID)
	}
	if got.Session.CWD != "/tmp" {
		t.Errorf("expected CWD '/tmp', got %q", got.Session.CWD)
	}
	if got.Event.EventType != "Stop" {
		t.Errorf("expected event type 'Stop', got %q", got.Event.EventType)
	}
}

func TestParsePayload_WhenGivenSessionStartEvent_ShouldPopulateSourceAndModel(t *testing.T) {
	input := `{
		"session_id":"sess-2",
		"cwd":"/home/user",
		"hook_event_name":"SessionStart",
		"source":"vscode",
		"model":"claude-3"
	}`
	got, err := ParsePayload([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Event.Source == nil || *got.Event.Source != "vscode" {
		t.Errorf("expected source 'vscode', got %v", got.Event.Source)
	}
	if got.Event.Model == nil || *got.Event.Model != "claude-3" {
		t.Errorf("expected model 'claude-3', got %v", got.Event.Model)
	}
}

func TestParsePayload_WhenGivenTranscriptPath_ShouldSetItOnSession(t *testing.T) {
	input := `{
		"session_id":"sess-3",
		"cwd":"/tmp",
		"hook_event_name":"Stop",
		"transcript_path":"/home/user/.claude/transcript.jsonl"
	}`
	got, err := ParsePayload([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Session.TranscriptPath != "/home/user/.claude/transcript.jsonl" {
		t.Errorf("expected transcript path, got %q", got.Session.TranscriptPath)
	}
}

func TestParsePayload_WhenGivenToolEvent_ShouldPopulateToolFields(t *testing.T) {
	input := `{
		"session_id":"sess-4",
		"cwd":"/tmp",
		"hook_event_name":"PostToolUse",
		"tool_name":"Bash",
		"tool_input":{"command":"ls"},
		"tool_use_id":"tu-1",
		"tool_response":{"output":"file.txt"}
	}`
	got, err := ParsePayload([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Event.ToolName == nil || *got.Event.ToolName != "Bash" {
		t.Errorf("expected tool_name 'Bash', got %v", got.Event.ToolName)
	}
	if got.Event.ToolUseID == nil || *got.Event.ToolUseID != "tu-1" {
		t.Errorf("expected tool_use_id 'tu-1', got %v", got.Event.ToolUseID)
	}
	if got.Event.ToolInput == nil {
		t.Fatal("expected tool_input to be set")
	}
	var m map[string]string
	if err := json.Unmarshal(got.Event.ToolInput, &m); err != nil {
		t.Fatalf("failed to unmarshal tool_input: %v", err)
	}
	if m["command"] != "ls" {
		t.Errorf("expected tool_input.command='ls', got %q", m["command"])
	}
	if got.Event.ToolResponse == nil {
		t.Fatal("expected tool_response to be set")
	}
}

func TestParsePayload_WhenGivenPromptEvent_ShouldPopulatePromptField(t *testing.T) {
	input := `{
		"session_id":"sess-5",
		"cwd":"/tmp",
		"hook_event_name":"UserPromptSubmit",
		"prompt":"help me fix the bug"
	}`
	got, err := ParsePayload([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Event.Prompt == nil || *got.Event.Prompt != "help me fix the bug" {
		t.Errorf("expected prompt, got %v", got.Event.Prompt)
	}
}

func TestParsePayload_WhenOptionalFieldsMissing_ShouldLeavePointersNil(t *testing.T) {
	input := `{"session_id":"sess-6","cwd":"/tmp","hook_event_name":"Stop"}`
	got, err := ParsePayload([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Event.ToolName != nil {
		t.Errorf("expected nil ToolName, got %v", got.Event.ToolName)
	}
	if got.Event.Source != nil {
		t.Errorf("expected nil Source, got %v", got.Event.Source)
	}
	if got.Event.Prompt != nil {
		t.Errorf("expected nil Prompt, got %v", got.Event.Prompt)
	}
	if got.Event.IsInterrupt != nil {
		t.Errorf("expected nil IsInterrupt, got %v", got.Event.IsInterrupt)
	}
	if got.Event.ToolInput != nil {
		t.Errorf("expected nil ToolInput, got %v", got.Event.ToolInput)
	}
	if got.Event.ToolResponse != nil {
		t.Errorf("expected nil ToolResponse, got %v", got.Event.ToolResponse)
	}
	if got.Event.PermissionSuggestions != nil {
		t.Errorf("expected nil PermissionSuggestions, got %v", got.Event.PermissionSuggestions)
	}
}

func TestParsePayload_WhenGivenBooleanFields_ShouldParseCorrectly(t *testing.T) {
	input := `{
		"session_id":"sess-7",
		"cwd":"/tmp",
		"hook_event_name":"Stop",
		"is_interrupt":true,
		"stop_hook_active":false
	}`
	got, err := ParsePayload([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Event.IsInterrupt == nil || *got.Event.IsInterrupt != true {
		t.Errorf("expected is_interrupt=true, got %v", got.Event.IsInterrupt)
	}
	if got.Event.StopHookActive == nil || *got.Event.StopHookActive != false {
		t.Errorf("expected stop_hook_active=false, got %v", got.Event.StopHookActive)
	}
}

func TestParsePayload_ShouldSetTimestampToNow(t *testing.T) {
	input := `{"session_id":"sess-8","cwd":"/tmp","hook_event_name":"Stop"}`
	got, err := ParsePayload([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Event.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if got.Session.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
}

func TestParsePayload_ShouldSetSessionIDOnEvent(t *testing.T) {
	input := `{"session_id":"sess-9","cwd":"/tmp","hook_event_name":"Stop"}`
	got, err := ParsePayload([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Event.SessionID != "sess-9" {
		t.Errorf("expected event session_id 'sess-9', got %q", got.Event.SessionID)
	}
}

// --- ParsePayload: invalid inputs ---

func TestParsePayload_WhenGivenEmptyJSON_ShouldReturnError(t *testing.T) {
	_, err := ParsePayload([]byte(`{}`))
	if err == nil {
		t.Fatal("expected error for empty JSON object")
	}
}

func TestParsePayload_WhenGivenMalformedJSON_ShouldReturnError(t *testing.T) {
	_, err := ParsePayload([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParsePayload_WhenMissingSessionID_ShouldReturnError(t *testing.T) {
	input := `{"cwd":"/tmp","hook_event_name":"Stop"}`
	_, err := ParsePayload([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing session_id")
	}
}

func TestParsePayload_WhenMissingCWD_ShouldReturnError(t *testing.T) {
	input := `{"session_id":"s1","hook_event_name":"Stop"}`
	_, err := ParsePayload([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing cwd")
	}
}

func TestParsePayload_WhenMissingHookEventName_ShouldReturnError(t *testing.T) {
	input := `{"session_id":"s1","cwd":"/tmp"}`
	_, err := ParsePayload([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing hook_event_name")
	}
}

func TestParsePayload_WhenGivenEmptyBytes_ShouldReturnError(t *testing.T) {
	_, err := ParsePayload([]byte{})
	if err == nil {
		t.Fatal("expected error for empty bytes")
	}
}

// --- ParsePayload: notification event ---

func TestParsePayload_WhenGivenNotificationEvent_ShouldPopulateNotificationFields(t *testing.T) {
	input := `{
		"session_id":"sess-10",
		"cwd":"/tmp",
		"hook_event_name":"Notification",
		"message":"Build complete",
		"title":"clog",
		"notification_type":"info"
	}`
	got, err := ParsePayload([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Event.Message == nil || *got.Event.Message != "Build complete" {
		t.Errorf("expected message 'Build complete', got %v", got.Event.Message)
	}
	if got.Event.Title == nil || *got.Event.Title != "clog" {
		t.Errorf("expected title 'clog', got %v", got.Event.Title)
	}
	if got.Event.NotificationType == nil || *got.Event.NotificationType != "info" {
		t.Errorf("expected notification_type 'info', got %v", got.Event.NotificationType)
	}
}

// --- ParsePayload: subagent event ---

func TestParsePayload_WhenGivenSubagentEvent_ShouldPopulateAgentFields(t *testing.T) {
	input := `{
		"session_id":"sess-11",
		"cwd":"/tmp",
		"hook_event_name":"SubagentStart",
		"agent_id":"agent-1",
		"agent_transcript_path":"/tmp/agent.jsonl"
	}`
	got, err := ParsePayload([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Event.AgentID == nil || *got.Event.AgentID != "agent-1" {
		t.Errorf("expected agent_id 'agent-1', got %v", got.Event.AgentID)
	}
	if got.Event.AgentTranscriptPath == nil || *got.Event.AgentTranscriptPath != "/tmp/agent.jsonl" {
		t.Errorf("expected agent_transcript_path, got %v", got.Event.AgentTranscriptPath)
	}
}

// --- ParsePayload: PreCompact event ---

func TestParsePayload_WhenGivenPreCompactEvent_ShouldPopulateCompactFields(t *testing.T) {
	input := `{
		"session_id":"sess-12",
		"cwd":"/tmp",
		"hook_event_name":"PreCompact",
		"trigger_type":"auto",
		"custom_instructions":"be concise"
	}`
	got, err := ParsePayload([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Event.TriggerType == nil || *got.Event.TriggerType != "auto" {
		t.Errorf("expected trigger_type 'auto', got %v", got.Event.TriggerType)
	}
	if got.Event.CustomInstructions == nil || *got.Event.CustomInstructions != "be concise" {
		t.Errorf("expected custom_instructions, got %v", got.Event.CustomInstructions)
	}
}

// --- ParsePayload: SessionEnd event ---

func TestParsePayload_WhenGivenSessionEndEvent_ShouldPopulateReasonField(t *testing.T) {
	input := `{
		"session_id":"sess-13",
		"cwd":"/tmp",
		"hook_event_name":"SessionEnd",
		"reason":"user_exit"
	}`
	got, err := ParsePayload([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Event.Reason == nil || *got.Event.Reason != "user_exit" {
		t.Errorf("expected reason 'user_exit', got %v", got.Event.Reason)
	}
}
