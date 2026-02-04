package embedding

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

// --- NewHTTP ---

func TestNewHTTP_ShouldReturnEmbedderWithCorrectDimension(t *testing.T) {
	emb := NewHTTP(Voyage, "test-key")
	if emb.Dimension() != 1024 {
		t.Errorf("expected dimension 1024, got %d", emb.Dimension())
	}
}

func TestNewHTTP_ShouldUseProvidedProvider(t *testing.T) {
	emb := NewHTTP(OpenAI, "test-key")
	if emb.provider.Name != "OpenAI" {
		t.Errorf("expected provider 'OpenAI', got %q", emb.provider.Name)
	}
	if emb.Dimension() != 1536 {
		t.Errorf("expected dimension 1536, got %d", emb.Dimension())
	}
}

// --- HTTPEmbedder.Embed with mock server ---

func TestEmbed_WhenServerReturnsValidResponse_ShouldReturnEmbeddings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request structure
		var req embeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if len(req.Input) != 2 {
			t.Errorf("expected 2 inputs, got %d", len(req.Input))
		}
		if req.Model != "test-model" {
			t.Errorf("expected model 'test-model', got %q", req.Model)
		}

		// Verify auth header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("expected 'Bearer test-key', got %q", auth)
		}

		resp := embeddingResponse{
			Data: []embeddingData{
				{Embedding: []float32{0.1, 0.2, 0.3}},
				{Embedding: []float32{0.4, 0.5, 0.6}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := Provider{
		Name:      "Test",
		Endpoint:  srv.URL,
		Model:     "test-model",
		Dimension: 3,
	}
	emb := NewHTTP(p, "test-key")

	vecs, err := emb.Embed([]string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}
	if len(vecs[0]) != 3 {
		t.Errorf("expected 3-dim vector, got %d", len(vecs[0]))
	}
	if vecs[0][0] != 0.1 {
		t.Errorf("expected vecs[0][0]=0.1, got %v", vecs[0][0])
	}
	if vecs[1][2] != 0.6 {
		t.Errorf("expected vecs[1][2]=0.6, got %v", vecs[1][2])
	}
}

func TestEmbed_WhenServerReturnsError_ShouldReturnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	p := Provider{Name: "Test", Endpoint: srv.URL, Model: "m", Dimension: 3}
	emb := NewHTTP(p, "key")

	_, err := emb.Embed([]string{"hello"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestEmbed_WhenServerReturnsInvalidJSON_ShouldReturnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	p := Provider{Name: "Test", Endpoint: srv.URL, Model: "m", Dimension: 3}
	emb := NewHTTP(p, "key")

	_, err := emb.Embed([]string{"hello"})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestEmbed_WhenServerUnreachable_ShouldReturnError(t *testing.T) {
	p := Provider{Name: "Test", Endpoint: "http://127.0.0.1:1", Model: "m", Dimension: 3}
	emb := NewHTTP(p, "key")

	_, err := emb.Embed([]string{"hello"})
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestEmbed_WhenGivenSingleText_ShouldReturnSingleVector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingResponse{
			Data: []embeddingData{
				{Embedding: []float32{1.0, 2.0}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := Provider{Name: "Test", Endpoint: srv.URL, Model: "m", Dimension: 2}
	emb := NewHTTP(p, "key")

	vecs, err := emb.Embed([]string{"one"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != 1 {
		t.Errorf("expected 1 vector, got %d", len(vecs))
	}
}

// --- NewFromEnv ---

func TestNewFromEnv_WhenNoProviderConfigured_ShouldReturnError(t *testing.T) {
	// Clear all provider env vars
	os.Unsetenv("OLLAMA_EMBED_MODEL")
	os.Unsetenv("OLLAMA_HOST")
	os.Unsetenv("VOYAGE_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")

	_, err := NewFromEnv()
	if err == nil {
		t.Fatal("expected error when no provider is configured")
	}
}

func TestNewFromEnv_WhenVoyageKeySet_ShouldReturnHTTPEmbedder(t *testing.T) {
	os.Unsetenv("OLLAMA_EMBED_MODEL")
	os.Unsetenv("OPENAI_API_KEY")
	t.Setenv("VOYAGE_API_KEY", "test-voyage-key")

	emb, err := NewFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	httpEmb, ok := emb.(*HTTPEmbedder)
	if !ok {
		t.Fatal("expected *HTTPEmbedder")
	}
	if httpEmb.provider.Name != "Voyage AI" {
		t.Errorf("expected Voyage AI provider, got %q", httpEmb.provider.Name)
	}
	if httpEmb.Dimension() != 1024 {
		t.Errorf("expected dimension 1024, got %d", httpEmb.Dimension())
	}
}

func TestNewFromEnv_WhenOpenAIKeySet_ShouldReturnHTTPEmbedder(t *testing.T) {
	os.Unsetenv("OLLAMA_EMBED_MODEL")
	os.Unsetenv("VOYAGE_API_KEY")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	emb, err := NewFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	httpEmb, ok := emb.(*HTTPEmbedder)
	if !ok {
		t.Fatal("expected *HTTPEmbedder")
	}
	if httpEmb.provider.Name != "OpenAI" {
		t.Errorf("expected OpenAI provider, got %q", httpEmb.provider.Name)
	}
}

func TestNewFromEnv_WhenVoyageAndOpenAIBothSet_ShouldPreferVoyage(t *testing.T) {
	os.Unsetenv("OLLAMA_EMBED_MODEL")
	t.Setenv("VOYAGE_API_KEY", "voyage-key")
	t.Setenv("OPENAI_API_KEY", "openai-key")

	emb, err := NewFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	httpEmb, ok := emb.(*HTTPEmbedder)
	if !ok {
		t.Fatal("expected *HTTPEmbedder")
	}
	if httpEmb.provider.Name != "Voyage AI" {
		t.Errorf("expected Voyage AI to take priority, got %q", httpEmb.provider.Name)
	}
}

func TestNewFromEnv_WhenOllamaModelSetButNoHost_ShouldReturnError(t *testing.T) {
	t.Setenv("OLLAMA_EMBED_MODEL", "nomic-embed-text")
	os.Unsetenv("OLLAMA_HOST")
	os.Unsetenv("VOYAGE_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")

	_, err := NewFromEnv()
	if err == nil {
		t.Fatal("expected error when OLLAMA_HOST is not set")
	}
}

// --- Provider values ---

func TestVoyageProvider_ShouldHaveCorrectDefaults(t *testing.T) {
	if Voyage.Name != "Voyage AI" {
		t.Errorf("expected name 'Voyage AI', got %q", Voyage.Name)
	}
	if Voyage.Model != "voyage-3-lite" {
		t.Errorf("expected model 'voyage-3-lite', got %q", Voyage.Model)
	}
	if Voyage.Dimension != 1024 {
		t.Errorf("expected dimension 1024, got %d", Voyage.Dimension)
	}
	if Voyage.EnvKey != "VOYAGE_API_KEY" {
		t.Errorf("expected env key 'VOYAGE_API_KEY', got %q", Voyage.EnvKey)
	}
}

func TestOpenAIProvider_ShouldHaveCorrectDefaults(t *testing.T) {
	if OpenAI.Name != "OpenAI" {
		t.Errorf("expected name 'OpenAI', got %q", OpenAI.Name)
	}
	if OpenAI.Model != "text-embedding-3-small" {
		t.Errorf("expected model 'text-embedding-3-small', got %q", OpenAI.Model)
	}
	if OpenAI.Dimension != 1536 {
		t.Errorf("expected dimension 1536, got %d", OpenAI.Dimension)
	}
	if OpenAI.EnvKey != "OPENAI_API_KEY" {
		t.Errorf("expected env key 'OPENAI_API_KEY', got %q", OpenAI.EnvKey)
	}
}
