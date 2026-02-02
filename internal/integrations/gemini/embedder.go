// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

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
	client *genai.Client
	model  string
}

// NewEmbedder creates a new Gemini embedder.
func NewEmbedder(apiKey string) (*Embedder, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &Embedder{
		client: client,
		model:  "gemini-embedding-001", // 768 dimensions
	}, nil
}

// Close closes the Gemini client.
func (e *Embedder) Close() error {
	return e.client.Close()
}

// Embed generates an embedding for a single text.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	em := e.client.EmbeddingModel(e.model)
	res, err := em.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	if res.Embedding == nil || len(res.Embedding.Values) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	return res.Embedding.Values, nil
}

// EmbedBatch generates embeddings for multiple texts.
func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts cannot be empty")
	}

	em := e.client.EmbeddingModel(e.model)

	// Convert texts to genai.Text
	parts := make([]genai.Part, len(texts))
	for i, text := range texts {
		parts[i] = genai.Text(text)
	}

	res, err := em.EmbedContentWithTitle(ctx, "", parts...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate batch embeddings: %w", err)
	}

	if res.Embedding == nil || len(res.Embedding.Values) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	// Note: Gemini returns a single embedding for batch requests
	// For individual embeddings, we need to call Embed for each text
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
	return 768 // gemini-embedding-001 produces 768-dimensional vectors
}
