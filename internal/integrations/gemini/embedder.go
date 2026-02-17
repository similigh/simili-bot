// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-17

// Package gemini provides Gemini AI integration for embeddings and LLM.
package gemini

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Embedder generates embeddings using Gemini.
type Embedder struct {
	client      *genai.Client
	model       string
	retryConfig RetryConfig
}

// NewEmbedder creates a new Gemini embedder.
func NewEmbedder(apiKey, model string) (*Embedder, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	if model == "" {
		model = "gemini-embedding-001" // Default to gemini-embedding-001 (3072-dim)
	}

	return &Embedder{
		client:      client,
		model:       model,
		retryConfig: DefaultRetryConfig(),
	}, nil
}

// Close closes the Gemini client.
func (e *Embedder) Close() error {
	return e.client.Close()
}

// Embed generates an embedding for a single text.
// It retries on transient errors (429/5xx) with exponential backoff.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	return withRetry(ctx, e.retryConfig, "Embed", func() ([]float32, error) {
		em := e.client.EmbeddingModel(e.model)
		res, err := em.EmbedContent(ctx, genai.Text(text))
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding: %w", err)
		}

		if res.Embedding == nil || len(res.Embedding.Values) == 0 {
			return nil, fmt.Errorf("empty embedding returned")
		}

		return res.Embedding.Values, nil
	})
}

// EmbedBatch generates embeddings for multiple texts.
// Note: Gemini API doesn't support true batch embedding, so this calls Embed for each text.
func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts cannot be empty")
	}

	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embedding, err := e.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

// Dimensions returns the dimensionality of the embeddings.
func (e *Embedder) Dimensions() int {
	return 3072 // gemini-embedding-001 produces 3072-dimensional vectors
}
