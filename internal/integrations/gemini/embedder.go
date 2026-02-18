// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-17

// Package gemini provides AI integration for embeddings and LLM.
//
// TODO(2026-02-16): This package is named "gemini" for historical reasons, but it
// now supports multiple providers (Gemini and OpenAI). Recommend renaming
// directory/package to provider-neutral naming (for example `internal/integrations/ai`).
package gemini

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Embedder generates embeddings using Gemini or OpenAI.
type Embedder struct {
	provider    Provider
	gemini      *genai.Client
	openAI      *http.Client
	apiKey      string
	model       string
	dimensions  atomic.Int32
	retryConfig RetryConfig
}

// NewEmbedder creates a new embedder.
func NewEmbedder(apiKey, model string) (*Embedder, error) {
	provider, resolvedKey, err := ResolveProvider(apiKey)
	if err != nil {
		return nil, err
	}

	e := &Embedder{
		provider:    provider,
		apiKey:      resolvedKey,
		retryConfig: DefaultRetryConfig(),
	}

	switch provider {
	case ProviderGemini:
		ctx := context.Background()
		client, err := genai.NewClient(ctx, option.WithAPIKey(resolvedKey))
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini client: %w", err)
		}
		e.gemini = client
		if strings.TrimSpace(model) == "" || isLikelyOpenAIEmbeddingModel(model) {
			model = "text-embedding-004"
		}
	case ProviderOpenAI:
		e.openAI = &http.Client{Timeout: 60 * time.Second}
		if strings.TrimSpace(model) == "" || isLikelyGeminiEmbeddingModel(model) {
			model = "text-embedding-3-small"
		}
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	e.model = strings.TrimSpace(model)
	e.dimensions.Store(int32(inferEmbeddingDimensions(provider, e.model)))

	return e, nil
}

// Close closes underlying provider clients.
func (e *Embedder) Close() error {
	if e.gemini != nil {
		return e.gemini.Close()
	}
	return nil
}

// Provider returns the resolved provider.
func (e *Embedder) Provider() string {
	return string(e.provider)
}

// Model returns the resolved model.
func (e *Embedder) Model() string {
	return e.model
}

// Embed generates an embedding for a single text.
// It retries on transient errors (429/5xx) with exponential backoff.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	switch e.provider {
	case ProviderGemini:
		return e.embedGemini(ctx, text)
	case ProviderOpenAI:
		return e.embedOpenAI(ctx, text)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", e.provider)
	}
}

func (e *Embedder) embedGemini(ctx context.Context, text string) ([]float32, error) {
	return withRetry(ctx, e.retryConfig, "Embed", func() ([]float32, error) {
		em := e.gemini.EmbeddingModel(e.model)
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

func (e *Embedder) embedOpenAI(ctx context.Context, text string) ([]float32, error) {
	return withRetry(ctx, e.retryConfig, "Embed", func() ([]float32, error) {
		req := struct {
			Model string `json:"model"`
			Input string `json:"input"`
		}{
			Model: e.model,
			Input: text,
		}

		var resp struct {
			Data []struct {
				Embedding []float64 `json:"embedding"`
			} `json:"data"`
		}

		if err := callOpenAIJSON(ctx, e.openAI, e.apiKey, "/v1/embeddings", req, &resp); err != nil {
			return nil, fmt.Errorf("failed to generate embedding: %w", err)
		}

		if len(resp.Data) == 0 || len(resp.Data[0].Embedding) == 0 {
			return nil, fmt.Errorf("empty embedding returned")
		}

		embedding := make([]float32, len(resp.Data[0].Embedding))
		for i, v := range resp.Data[0].Embedding {
			embedding[i] = float32(v)
		}

		// Keep the dimensions aligned with provider output if model mapping is unknown.
		e.dimensions.Store(int32(len(embedding)))
		return embedding, nil
	})
}

// EmbedBatch generates embeddings for multiple texts.
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
	return int(e.dimensions.Load())
}

func inferEmbeddingDimensions(provider Provider, model string) int {
	m := strings.ToLower(strings.TrimSpace(model))

	switch provider {
	case ProviderOpenAI:
		switch {
		case strings.Contains(m, "text-embedding-3-large"):
			return 3072
		case strings.Contains(m, "text-embedding-3-small"), strings.Contains(m, "text-embedding-ada-002"):
			return 1536
		default:
			return 1536
		}
	case ProviderGemini:
		switch {
		case strings.Contains(m, "gemini-embedding-001"):
			return 3072
		case strings.Contains(m, "text-embedding-004"), strings.Contains(m, "text-embedding-005"):
			return 768
		default:
			return 768
		}
	default:
		return 0
	}
}

func isLikelyGeminiEmbeddingModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(m, "gemini") || strings.Contains(m, "text-embedding-004") || strings.Contains(m, "text-embedding-005")
}

func isLikelyOpenAIEmbeddingModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(m, "text-embedding-3") || strings.Contains(m, "text-embedding-ada")
}
