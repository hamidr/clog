package embedding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPEmbedder calls an OpenAI-compatible embeddings API.
type HTTPEmbedder struct {
	provider Provider
	apiKey   string
	client   *http.Client
}

// NewHTTP creates an embedder for the given provider and API key.
func NewHTTP(provider Provider, apiKey string) *HTTPEmbedder {
	return &HTTPEmbedder{
		provider: provider,
		apiKey:   apiKey,
		client:   &http.Client{Timeout: 60 * time.Second},
	}
}

func (e *HTTPEmbedder) Dimension() int { return e.provider.Dimension }

// Embed sends texts to the embedding API and returns the resulting vectors.
func (e *HTTPEmbedder) Embed(texts []string) ([][]float32, error) {
	reqBody, err := json.Marshal(embeddingRequest{
		Input: texts,
		Model: e.provider.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, e.provider.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s: %w", e.provider.Name, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s API returned %d: %s", e.provider.Name, resp.StatusCode, truncate(string(body), 200))
	}

	var result embeddingResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	embeddings := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		embeddings[i] = d.Embedding
	}
	return embeddings, nil
}

type embeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type embeddingResponse struct {
	Data []embeddingData `json:"data"`
}

type embeddingData struct {
	Embedding []float32 `json:"embedding"`
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
