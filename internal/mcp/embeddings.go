package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Embedder interface for generating embeddings
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// OpenAIEmbedder uses OpenAI's embedding API
type OpenAIEmbedder struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// NewOpenAIEmbedder creates a new OpenAI embedder
func NewOpenAIEmbedder(apiKey string) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		apiKey:  apiKey,
		model:   "text-embedding-3-small", // 1536 dimensions
		baseURL: "https://api.openai.com/v1",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewOpenAIEmbedderWithBaseURL creates an embedder with custom base URL
func NewOpenAIEmbedderWithBaseURL(apiKey, baseURL, model string) *OpenAIEmbedder {
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &OpenAIEmbedder{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Embed generates an embedding for a single text
func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts
func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := map[string]any{
		"model": e.model,
		"input": texts,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Convert to float32 and order by index
	embeddings := make([][]float32, len(result.Data))
	for _, d := range result.Data {
		embedding := make([]float32, len(d.Embedding))
		for i, v := range d.Embedding {
			embedding[i] = float32(v)
		}
		embeddings[d.Index] = embedding
	}

	return embeddings, nil
}

// LocalEmbedder uses a local embedding model (placeholder)
type LocalEmbedder struct {
	dimension int
}

// NewLocalEmbedder creates a local embedder (for testing/development)
func NewLocalEmbedder(dimension int) *LocalEmbedder {
	return &LocalEmbedder{dimension: dimension}
}

// Embed generates a simple hash-based embedding (for testing only)
func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Simple hash-based embedding for testing
	embedding := make([]float32, e.dimension)
	hash := 0
	for _, c := range text {
		hash = hash*31 + int(c)
	}

	for i := range embedding {
		hash = hash*1103515245 + 12345
		embedding[i] = float32(hash%1000) / 1000.0
	}

	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts
func (e *LocalEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

// BedrockEmbedder uses AWS Bedrock for embeddings
type BedrockEmbedder struct {
	region    string
	modelID   string
	accessKey string
	secretKey string
}

// NewBedrockEmbedder creates a new Bedrock embedder
func NewBedrockEmbedder(region, accessKey, secretKey string) *BedrockEmbedder {
	return &BedrockEmbedder{
		region:    region,
		modelID:   "amazon.titan-embed-text-v1",
		accessKey: accessKey,
		secretKey: secretKey,
	}
}

// Embed generates an embedding using Bedrock
func (e *BedrockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Bedrock implementation would go here
	// For now, return a placeholder
	return nil, fmt.Errorf("Bedrock embeddings not yet implemented")
}

// EmbedBatch generates embeddings for multiple texts
func (e *BedrockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

// OllamaEmbedder uses Ollama for local embeddings
type OllamaEmbedder struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewOllamaEmbedder creates a new Ollama embedder
func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Embed generates an embedding using Ollama
func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]any{
		"model":  e.model,
		"prompt": text,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/api/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Embedding []float64 `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	embedding := make([]float32, len(result.Embedding))
	for i, v := range result.Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts
func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}
