package summary

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"clog/internal/model"
)

// --- Provider auto-detection ---

func TestNewFromEnv_WhenNoProviderConfigured_ShouldReturnNil(t *testing.T) {
	os.Unsetenv("OLLAMA_CHAT_MODEL")
	os.Unsetenv("OLLAMA_HOST")
	os.Unsetenv("OPENAI_API_KEY")

	s, err := NewFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != nil {
		t.Error("expected nil summarizer when no provider is configured")
	}
}

func TestNewFromEnv_WhenOllamaChatModelAndHostSet_ShouldReturnOllamaSummarizer(t *testing.T) {
	t.Setenv("OLLAMA_CHAT_MODEL", "llama3.2")
	t.Setenv("OLLAMA_HOST", "http://localhost:11434")
	os.Unsetenv("OPENAI_API_KEY")

	s, err := NewFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil summarizer")
	}
	if s.Model() != "llama3.2" {
		t.Errorf("expected model 'llama3.2', got %q", s.Model())
	}
	if !strings.Contains(s.endpoint, "localhost:11434") {
		t.Errorf("expected Ollama endpoint, got %q", s.endpoint)
	}
}

func TestNewFromEnv_WhenOllamaChatModelSetButNoHost_ShouldReturnNil(t *testing.T) {
	t.Setenv("OLLAMA_CHAT_MODEL", "llama3.2")
	os.Unsetenv("OLLAMA_HOST")
	os.Unsetenv("OPENAI_API_KEY")

	s, err := NewFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != nil {
		t.Error("expected nil when OLLAMA_HOST is missing")
	}
}

func TestNewFromEnv_WhenOpenAIKeySet_ShouldReturnOpenAISummarizer(t *testing.T) {
	os.Unsetenv("OLLAMA_CHAT_MODEL")
	t.Setenv("OPENAI_API_KEY", "sk-test")

	s, err := NewFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil summarizer")
	}
	if s.Model() != "gpt-4o-mini" {
		t.Errorf("expected model 'gpt-4o-mini', got %q", s.Model())
	}
}

func TestNewFromEnv_WhenBothOllamaAndOpenAISet_ShouldPreferOllama(t *testing.T) {
	t.Setenv("OLLAMA_CHAT_MODEL", "llama3.2")
	t.Setenv("OLLAMA_HOST", "http://localhost:11434")
	t.Setenv("OPENAI_API_KEY", "sk-test")

	s, err := NewFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil summarizer")
	}
	if s.Model() != "llama3.2" {
		t.Errorf("expected Ollama to take priority, got model %q", s.Model())
	}
}

// --- Summarize behavior ---

func TestSummarize_WhenServerReturnsValidResponse_ShouldReturnSummaryText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if len(req.Messages) != 2 {
			t.Errorf("expected 2 messages (system + user), got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("expected system message first, got %q", req.Messages[0].Role)
		}

		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: "Added auth to the login flow."}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	s := &Summarizer{
		endpoint: srv.URL,
		model:    "test-model",
		apiKey:   "test-key",
		client:   http.DefaultClient,
	}

	messages := []model.StoredMessage{
		{Role: "user", Content: "Add authentication", Timestamp: time.Now()},
		{Role: "assistant", Content: "I'll add JWT auth.", Timestamp: time.Now()},
	}

	text, err := s.Summarize(messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "Added auth to the login flow." {
		t.Errorf("expected summary text, got %q", text)
	}
}

func TestSummarize_WhenServerReturnsError_ShouldReturnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"overloaded"}`))
	}))
	defer srv.Close()

	s := &Summarizer{
		endpoint: srv.URL,
		model:    "test-model",
		apiKey:   "test-key",
		client:   http.DefaultClient,
	}

	messages := []model.StoredMessage{
		{Role: "user", Content: "hello", Timestamp: time.Now()},
		{Role: "assistant", Content: "hi", Timestamp: time.Now()},
	}

	_, err := s.Summarize(messages)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSummarize_WhenServerReturnsNoChoices_ShouldReturnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{Choices: []chatChoice{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	s := &Summarizer{
		endpoint: srv.URL,
		model:    "test-model",
		apiKey:   "test-key",
		client:   http.DefaultClient,
	}

	messages := []model.StoredMessage{
		{Role: "user", Content: "hello", Timestamp: time.Now()},
	}

	_, err := s.Summarize(messages)
	if err == nil {
		t.Fatal("expected error when API returns no choices")
	}
}

func TestSummarize_WhenServerUnreachable_ShouldReturnError(t *testing.T) {
	s := &Summarizer{
		endpoint: "http://127.0.0.1:1",
		model:    "test-model",
		apiKey:   "test-key",
		client:   &http.Client{Timeout: 1 * time.Second},
	}

	messages := []model.StoredMessage{
		{Role: "user", Content: "hello", Timestamp: time.Now()},
	}

	_, err := s.Summarize(messages)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// --- Prompt construction ---

func TestSummarize_ShouldSendConversationAsUserMessage(t *testing.T) {
	var receivedContent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedContent = req.Messages[1].Content

		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: "summary"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	s := &Summarizer{
		endpoint: srv.URL,
		model:    "m",
		apiKey:   "k",
		client:   http.DefaultClient,
	}

	messages := []model.StoredMessage{
		{Role: "user", Content: "Fix the bug", Timestamp: time.Now()},
		{Role: "assistant", Content: "Done, fixed it.", Timestamp: time.Now()},
	}

	s.Summarize(messages)

	if !strings.Contains(receivedContent, "User: Fix the bug") {
		t.Errorf("expected user message in prompt, got %q", receivedContent)
	}
	if !strings.Contains(receivedContent, "Assistant: Done, fixed it.") {
		t.Errorf("expected assistant message in prompt, got %q", receivedContent)
	}
}

func TestSummarize_WhenConversationExceedsLimit_ShouldTruncate(t *testing.T) {
	var receivedContent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedContent = req.Messages[1].Content

		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: "summary"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	s := &Summarizer{
		endpoint: srv.URL,
		model:    "m",
		apiKey:   "k",
		client:   http.DefaultClient,
	}

	// Create messages that exceed maxConversationChars
	var messages []model.StoredMessage
	longContent := strings.Repeat("x", 2000)
	for i := 0; i < 10; i++ {
		messages = append(messages, model.StoredMessage{
			Role: "user", Content: longContent, Timestamp: time.Now(),
		})
	}

	s.Summarize(messages)

	if !strings.Contains(receivedContent, "... (truncated)") {
		t.Error("expected truncation marker in prompt for oversized conversation")
	}
	if len(receivedContent) > maxConversationChars+200 {
		t.Errorf("prompt content too large: %d chars", len(receivedContent))
	}
}

func TestSummarize_ShouldSendAuthorizationHeader(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: "ok"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	s := &Summarizer{
		endpoint: srv.URL,
		model:    "m",
		apiKey:   "my-secret-key",
		client:   http.DefaultClient,
	}

	s.Summarize([]model.StoredMessage{
		{Role: "user", Content: "hi", Timestamp: time.Now()},
	})

	if receivedAuth != "Bearer my-secret-key" {
		t.Errorf("expected 'Bearer my-secret-key', got %q", receivedAuth)
	}
}
