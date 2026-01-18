package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/pgvector/pgvector-go"
	"modelgate/internal/domain"
)

// EmbeddingService generates embeddings for semantic caching
type EmbeddingService struct {
	client EmbeddingClient
	model  string
}

// EmbeddingClient interface for generating embeddings
type EmbeddingClient interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(client EmbeddingClient, model string) *EmbeddingService {
	if model == "" {
		model = "nomic-embed-text" // Default Ollama model
	}
	return &EmbeddingService{
		client: client,
		model:  model,
	}
}

// GenerateEmbedding creates an embedding vector for a prompt
func (s *EmbeddingService) GenerateEmbedding(ctx context.Context, prompt string) (pgvector.Vector, error) {
	if s.client == nil {
		return pgvector.Vector{}, fmt.Errorf("embedding client not configured")
	}

	embeddings, err := s.client.Embed(ctx, []string{prompt})
	if err != nil {
		return pgvector.Vector{}, fmt.Errorf("failed to generate embedding: %w", err)
	}

	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return pgvector.Vector{}, fmt.Errorf("empty embedding returned")
	}

	return pgvector.NewVector(embeddings[0]), nil
}

// HashPrompt generates a SHA256 hash for exact match fast path
func HashPrompt(prompt string) string {
	hash := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(hash[:])
}

// NormalizePrompt normalizes messages into a consistent string for hashing
func NormalizePrompt(messages []domain.Message) string {
	// Only cache based on the LAST user message (current query)
	// Not the entire conversation history
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == "user" {
			var textContent string
			for _, block := range msg.Content {
				if block.Type == "text" {
					textContent += strings.TrimSpace(block.Text)
				}
			}
			return "user:" + textContent
		}
	}

	return ""
}
