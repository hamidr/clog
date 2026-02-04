// Package summary generates LLM-based session summaries via chat completion APIs.
package summary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"clog/internal/model"
)

const maxConversationChars = 8000

const systemPrompt = "Summarize this Claude Code session in one concise paragraph. Focus on what was accomplished, not the process. Do not use markdown formatting in the summary."

// Summarizer calls an OpenAI-compatible chat completion API.
type Summarizer struct {
	endpoint string
	model    string
	apiKey   string
	client   *http.Client
}

// NewFromEnv detects a chat completion provider from the environment.
// Returns (nil, nil) when no provider is configured — this is not an error.
func NewFromEnv() (*Summarizer, error) {
	if model := os.Getenv("OLLAMA_CHAT_MODEL"); model != "" {
		host := os.Getenv("OLLAMA_HOST")
		if host == "" {
			return nil, nil
		}
		return &Summarizer{
			endpoint: strings.TrimRight(host, "/") + "/v1/chat/completions",
			model:    model,
			apiKey:   "ollama",
			client:   &http.Client{Timeout: 120 * time.Second},
		}, nil
	}

	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return &Summarizer{
			endpoint: "https://api.openai.com/v1/chat/completions",
			model:    "gpt-4o-mini",
			apiKey:   key,
			client:   &http.Client{Timeout: 60 * time.Second},
		}, nil
	}

	return nil, nil
}

// Model returns the model name used for summarization.
func (s *Summarizer) Model() string { return s.model }

// Summarize generates a one-paragraph summary of the given messages.
func (s *Summarizer) Summarize(messages []model.StoredMessage) (string, error) {
	chatMsgs := buildPrompt(messages)

	reqBody, err := json.Marshal(chatRequest{
		Model:    s.model,
		Messages: chatMsgs,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, s.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat completion request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return "", fmt.Errorf("chat API returned %d: %s", resp.StatusCode, preview)
	}

	var result chatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("chat API returned no choices")
	}

	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func buildPrompt(messages []model.StoredMessage) []chatMessage {
	var sb strings.Builder
	for _, m := range messages {
		label := "User"
		if m.Role == "assistant" {
			label = "Assistant"
		}
		line := fmt.Sprintf("%s: %s\n\n", label, m.Content)
		if sb.Len()+len(line) > maxConversationChars {
			sb.WriteString("... (truncated)\n")
			break
		}
		sb.WriteString(line)
	}

	return []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: sb.String()},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}
