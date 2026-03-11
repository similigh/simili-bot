// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-03-05
// Last Modified: 2026-03-05

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestInferEmbeddingDimensions(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		model    string
		want     int
	}{
		{
			name:     "gemini-embedding-001 → 3072",
			provider: ProviderGemini,
			model:    "gemini-embedding-001",
			want:     3072,
		},
		{
			name:     "unknown gemini model defaults to 3072",
			provider: ProviderGemini,
			model:    "gemini-embedding-future",
			want:     3072,
		},
		{
			name:     "openai text-embedding-3-small → 1536",
			provider: ProviderOpenAI,
			model:    "text-embedding-3-small",
			want:     1536,
		},
		{
			name:     "openai text-embedding-ada-002 → 1536",
			provider: ProviderOpenAI,
			model:    "text-embedding-ada-002",
			want:     1536,
		},
		{
			name:     "openai text-embedding-3-large → 3072",
			provider: ProviderOpenAI,
			model:    "text-embedding-3-large",
			want:     3072,
		},
		{
			name:     "unknown openai model defaults to 1536",
			provider: ProviderOpenAI,
			model:    "text-embedding-unknown",
			want:     1536,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferEmbeddingDimensions(tt.provider, tt.model)
			if got != tt.want {
				t.Errorf("inferEmbeddingDimensions(%s, %q) = %d, want %d", tt.provider, tt.model, got, tt.want)
			}
		})
	}
}

func TestIsLikelyGeminiEmbeddingModel(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"gemini-embedding-001", true},
		{"gemini-embedding-future", true},
		{"text-embedding-004", true},  // legacy — still recognised to prevent OpenAI forwarding
		{"text-embedding-005", true},  // legacy — still recognised to prevent OpenAI forwarding
		{"text-embedding-3-small", false},
		{"text-embedding-ada-002", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := isLikelyGeminiEmbeddingModel(tt.model)
			if got != tt.want {
				t.Errorf("isLikelyGeminiEmbeddingModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestEmbedBatch_EmptyInput(t *testing.T) {
	srv, _ := statusServer([]int{200}, func(_ int) []byte { return embeddingOKBody() })
	defer srv.Close()

	e := newTestEmbedder(srv.URL)
	_, err := e.EmbedBatch(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestEmbedBatch_ResultsPreserveOrder(t *testing.T) {
	// The handler parses the input text ("text-N") and encodes N+1 as the first
	// embedding value. This lets us deterministically verify that results[i]
	// corresponds to texts[i] regardless of goroutine scheduling order.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var idx int
		fmt.Sscanf(req.Input, "text-%d", &idx)

		type embItem struct {
			Embedding []float64 `json:"embedding"`
		}
		type embResp struct {
			Data []embItem `json:"data"`
		}
		b, _ := json.Marshal(embResp{Data: []embItem{{Embedding: []float64{float64(idx + 1), 0.0, 0.0}}}})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	e := newTestEmbedder(srv.URL)
	e.retryConfig = fastRetry

	texts := make([]string, 20)
	for i := range texts {
		texts[i] = fmt.Sprintf("text-%d", i)
	}

	results, err := e.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != len(texts) {
		t.Fatalf("expected %d results, got %d", len(texts), len(results))
	}
	for i, emb := range results {
		if len(emb) == 0 {
			t.Errorf("result[%d] is empty", i)
			continue
		}
		// texts[i] = "text-i", so the handler encodes i+1 as emb[0].
		if got, want := emb[0], float32(i+1); got != want {
			t.Errorf("result[%d][0] = %v, want %v (order not preserved)", i, got, want)
		}
	}
}

func TestEmbedBatch_PropagatesError(t *testing.T) {
	// First request succeeds, subsequent ones fail with a non-retryable 400.
	var counter atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := counter.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			w.WriteHeader(200)
			_, _ = w.Write(embeddingOKBody())
		} else {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
		}
	}))
	defer srv.Close()

	e := newTestEmbedder(srv.URL)
	e.retryConfig = fastRetry

	_, err := e.EmbedBatch(context.Background(), []string{"a", "b", "c"})
	if err == nil {
		t.Fatal("expected error from failed embedding, got nil")
	}
}

func TestEmbedBatch_ConcurrencyLimit(t *testing.T) {
	// Verify that EmbedBatch honours maxBatchConcurrency: concurrent in-flight
	// requests should never exceed the limit.
	var (
		inFlight    atomic.Int32
		maxObserved atomic.Int32
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := inFlight.Add(1)
		defer inFlight.Add(-1)
		// Update max observed — simple CAS loop.
		for {
			old := maxObserved.Load()
			if cur <= old || maxObserved.CompareAndSwap(old, cur) {
				break
			}
		}
		// Hold the request briefly so multiple goroutines overlap in-flight,
		// making the concurrency cap observable.
		time.Sleep(5 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(embeddingOKBody())
	}))
	defer srv.Close()

	e := newTestEmbedder(srv.URL)
	e.retryConfig = fastRetry

	texts := make([]string, maxBatchConcurrency*3)
	for i := range texts {
		texts[i] = fmt.Sprintf("text-%d", i)
	}

	if _, err := e.EmbedBatch(context.Background(), texts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := maxObserved.Load(); got > int32(maxBatchConcurrency) {
		t.Errorf("max concurrent requests = %d, want <= %d", got, maxBatchConcurrency)
	}
}

func TestNewEmbedderRejectsLegacyGeminiModels(t *testing.T) {
	// Fake a Gemini API key so provider resolution picks Gemini.
	t.Setenv("GEMINI_API_KEY", "fake-key-for-test")

	for _, model := range []string{"text-embedding-004", "text-embedding-005"} {
		t.Run(model, func(t *testing.T) {
			_, err := NewEmbedder("fake-key-for-test", model)
			if err == nil {
				t.Fatalf("Expected error for deprecated model %q, got nil", model)
			}
			if !strings.Contains(err.Error(), "gemini-embedding-001") {
				t.Errorf("Error message should mention gemini-embedding-001, got: %v", err)
			}
		})
	}
}
