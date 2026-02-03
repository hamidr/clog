// Package model defines the domain types shared across the application.
package model

import (
	"encoding/json"
	"time"
)

// Session represents a Claude Code session.
type Session struct {
	ID             string
	CWD            string
	TranscriptPath string
	CreatedAt      time.Time
}

// Event represents a single hook event with a superset of fields.
// Each event type populates only its relevant fields; the rest remain nil.
type Event struct {
	SessionID      string
	EventType      string
	Timestamp      time.Time
	PermissionMode *string

	// SessionStart
	Source    *string
	Model    *string
	AgentType *string

	// UserPromptSubmit
	Prompt *string

	// Tool-related
	ToolName              *string
	ToolInput             json.RawMessage
	ToolUseID             *string
	ToolResponse          json.RawMessage
	PermissionSuggestions json.RawMessage
	Error                 *string
	IsInterrupt           *bool

	// Notification
	Message          *string
	Title            *string
	NotificationType *string

	// Subagent
	AgentID             *string
	AgentTranscriptPath *string
	StopHookActive      *bool

	// PreCompact
	TriggerType        *string
	CustomInstructions *string

	// SessionEnd
	Reason *string
}

// Message represents a conversation message extracted from a transcript.
type Message struct {
	SessionID  string
	UUID       string
	ParentUUID string
	Role       string
	Content    string
	RawContent string
	Model      string
	Timestamp  time.Time
}

// StoredMessage is a persisted message with its database ID.
type StoredMessage struct {
	ID        int64
	SessionID string
	Role      string
	Content   string
	Timestamp time.Time
}

// SearchResult pairs a message with a similarity score.
type SearchResult struct {
	ID        int64
	SessionID string
	Role      string
	Content   string
	Score     float64
	Timestamp time.Time
}

// ToolResult represents a tool call event from the events table.
type ToolResult struct {
	SessionID    string
	ToolName     string
	ToolInput    string // raw JSON
	ToolResponse string // raw JSON
	Timestamp    time.Time
}

// HarvestResult holds parsed messages and the new file read offset.
type HarvestResult struct {
	Messages  []Message
	NewOffset int64
}
