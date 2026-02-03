// Package transcript parses Claude Code JSONL transcript files.
package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"clog/internal/model"
)

// transcriptLine is the JSON structure of a single JSONL line.
type transcriptLine struct {
	Type    string          `json:"type"`
	UUID    string          `json:"uuid"`
	Parent  string          `json:"parentUuid"`
	Message *messagePayload `json:"message"`
	// Some lines use a top-level timestamp.
	Timestamp string `json:"timestamp"`
}

type messagePayload struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Model   string          `json:"model"`
}

// Harvest reads new lines from the transcript starting at fromOffset,
// parses user and assistant messages, and returns them with the new offset.
func Harvest(sessionID, transcriptPath string, fromOffset int64) (*model.HarvestResult, error) {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", transcriptPath, err)
	}
	defer f.Close()

	if fromOffset > 0 {
		if _, err := f.Seek(fromOffset, io.SeekStart); err != nil {
			return nil, fmt.Errorf("seek to %d: %w", fromOffset, err)
		}
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10 MB max line

	now := time.Now().UTC()
	var messages []model.Message

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var tl transcriptLine
		if err := json.Unmarshal(line, &tl); err != nil {
			continue
		}
		if tl.Message == nil {
			continue
		}

		role := tl.Message.Role
		if role != "user" && role != "assistant" {
			continue
		}

		ts := parseTimestamp(tl.Timestamp, now)

		messages = append(messages, model.Message{
			SessionID:  sessionID,
			UUID:       tl.UUID,
			ParentUUID: tl.Parent,
			Role:       role,
			Content:    extractText(tl.Message.Content),
			RawContent: string(tl.Message.Content),
			Model:      tl.Message.Model,
			Timestamp:  ts,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan transcript: %w", err)
	}

	newOffset, _ := f.Seek(0, io.SeekCurrent)

	return &model.HarvestResult{
		Messages:  messages,
		NewOffset: newOffset,
	}, nil
}

// extractText pulls human-readable text from a message's content field.
// User messages have a plain string; assistant messages have an array of blocks.
func extractText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Try plain string first (user messages).
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Try array of content blocks (assistant messages).
	var blocks []json.RawMessage
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return string(raw)
	}

	var parts []string
	for _, block := range blocks {
		var obj struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(block, &obj); err != nil {
			continue
		}
		if obj.Type == "text" && obj.Text != "" {
			parts = append(parts, obj.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func parseTimestamp(raw string, fallback time.Time) time.Time {
	if raw == "" {
		return fallback
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t
	}
	return fallback
}
