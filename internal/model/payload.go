package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// ParsedPayload groups the session and event derived from a hook's JSON input.
type ParsedPayload struct {
	Session Session
	Event   Event
}

// hookPayload mirrors the raw JSON schema sent by Claude Code hooks.
type hookPayload struct {
	SessionID             string           `json:"session_id"`
	CWD                   string           `json:"cwd"`
	HookEventName         string           `json:"hook_event_name"`
	TranscriptPath        string           `json:"transcript_path"`
	PermissionMode        *string          `json:"permission_mode"`
	Source                *string          `json:"source"`
	Model                 *string          `json:"model"`
	AgentType             *string          `json:"agent_type"`
	Prompt                *string          `json:"prompt"`
	ToolName              *string          `json:"tool_name"`
	ToolInput             *json.RawMessage `json:"tool_input"`
	ToolUseID             *string          `json:"tool_use_id"`
	ToolResponse          *json.RawMessage `json:"tool_response"`
	PermissionSuggestions *json.RawMessage `json:"permission_suggestions"`
	Error                 *string          `json:"error"`
	IsInterrupt           *bool            `json:"is_interrupt"`
	Message               *string          `json:"message"`
	Title                 *string          `json:"title"`
	NotificationType      *string          `json:"notification_type"`
	AgentID               *string          `json:"agent_id"`
	AgentTranscriptPath   *string          `json:"agent_transcript_path"`
	StopHookActive        *bool            `json:"stop_hook_active"`
	TriggerType           *string          `json:"trigger_type"`
	CustomInstructions    *string          `json:"custom_instructions"`
	Reason                *string          `json:"reason"`
}

// ParsePayload converts raw JSON bytes into domain types.
func ParsePayload(data []byte) (*ParsedPayload, error) {
	var p hookPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	if p.SessionID == "" || p.CWD == "" || p.HookEventName == "" {
		return nil, fmt.Errorf("missing required fields (session_id, cwd, hook_event_name)")
	}

	now := time.Now().UTC()

	session := Session{
		ID:             p.SessionID,
		CWD:            p.CWD,
		TranscriptPath: p.TranscriptPath,
		CreatedAt:      now,
	}

	event := Event{
		SessionID:           p.SessionID,
		EventType:           p.HookEventName,
		Timestamp:           now,
		PermissionMode:      p.PermissionMode,
		Source:              p.Source,
		Model:               p.Model,
		AgentType:           p.AgentType,
		Prompt:              p.Prompt,
		ToolName:            p.ToolName,
		ToolUseID:           p.ToolUseID,
		Error:               p.Error,
		IsInterrupt:         p.IsInterrupt,
		Message:             p.Message,
		Title:               p.Title,
		NotificationType:    p.NotificationType,
		AgentID:             p.AgentID,
		AgentTranscriptPath: p.AgentTranscriptPath,
		StopHookActive:      p.StopHookActive,
		TriggerType:         p.TriggerType,
		CustomInstructions:  p.CustomInstructions,
		Reason:              p.Reason,
	}

	if p.ToolInput != nil {
		event.ToolInput = json.RawMessage(*p.ToolInput)
	}
	if p.ToolResponse != nil {
		event.ToolResponse = json.RawMessage(*p.ToolResponse)
	}
	if p.PermissionSuggestions != nil {
		event.PermissionSuggestions = json.RawMessage(*p.PermissionSuggestions)
	}

	return &ParsedPayload{Session: session, Event: event}, nil
}
