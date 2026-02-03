// Package embedding provides vector embedding generation for semantic search.
package embedding

import (
	"fmt"
	"os"
	"strings"
)

// Embedder generates vector embeddings from text.
type Embedder interface {
	// Embed returns one embedding per input text.
	Embed(texts []string) ([][]float32, error)

	// Dimension returns the embedding vector length.
	Dimension() int
}

// Provider identifies a supported embedding API.
type Provider struct {
	Name      string
	Endpoint  string
	Model     string
	Dimension int
	EnvKey    string
}

var (
	Voyage = Provider{
		Name:      "Voyage AI",
		Endpoint:  "https://api.voyageai.com/v1/embeddings",
		Model:     "voyage-3-lite",
		Dimension: 1024,
		EnvKey:    "VOYAGE_API_KEY",
	}

	OpenAI = Provider{
		Name:      "OpenAI",
		Endpoint:  "https://api.openai.com/v1/embeddings",
		Model:     "text-embedding-3-small",
		Dimension: 1536,
		EnvKey:    "OPENAI_API_KEY",
	}
)

// NewFromEnv detects an embedding provider from the environment.
// It checks OLLAMA_EMBED_MODEL first, then VOYAGE_API_KEY, then OPENAI_API_KEY.
func NewFromEnv() (Embedder, error) {
	if model := os.Getenv("OLLAMA_EMBED_MODEL"); model != "" {
		return newOllama(model)
	}
	for _, p := range []Provider{Voyage, OpenAI} {
		if key := os.Getenv(p.EnvKey); key != "" {
			return NewHTTP(p, key), nil
		}
	}
	return nil, fmt.Errorf("no embedding provider found; set OLLAMA_EMBED_MODEL, VOYAGE_API_KEY, or OPENAI_API_KEY")
}

// newOllama creates an Embedder backed by a local Ollama instance.
// It probes the model with a short string to discover the embedding dimension.
func newOllama(model string) (Embedder, error) {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		return nil, fmt.Errorf("OLLAMA_HOST is not set")
	}

	p := Provider{
		Name:     "Ollama",
		Endpoint: strings.TrimRight(host, "/") + "/v1/embeddings",
		Model:    model,
	}

	emb := NewHTTP(p, "ollama") // Ollama ignores the auth header

	// Probe to discover the embedding dimension.
	vecs, err := emb.Embed([]string{"hello"})
	if err != nil {
		return nil, fmt.Errorf("ollama probe (%s): %w", model, err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil, fmt.Errorf("ollama probe returned empty embedding")
	}

	emb.provider.Dimension = len(vecs[0])
	return emb, nil
}
