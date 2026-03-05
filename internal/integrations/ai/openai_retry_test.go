// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-03-05
// Last Modified: 2026-03-05

package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// fastRetry is used in all tests to keep them quick.
var fastRetry = RetryConfig{
	MaxRetries:  2,
	BaseDelay:   1 * time.Millisecond,
	MaxDelay:    10 * time.Millisecond,
	JitterRatio: 0,
}

// newEmbedder builds a test Embedder pointed at the given httptest server URL.
func newTestEmbedder(srvURL string) *Embedder {
	e := &Embedder{
		provider:    ProviderOpenAI,
		openAI:      &http.Client{},
		apiKey:      "test-key",
		model:       "text-embedding-3-small",
		baseURL:     srvURL,
		retryConfig: fastRetry,
	}
	e.dimensions.Store(1536)
	return e
}

// newTestLLMClient builds a test LLMClient pointed at the given httptest server URL.
func newTestLLMClient(srvURL string) *LLMClient {
	return &LLMClient{
		provider:    ProviderOpenAI,
		openAI:      &http.Client{},
		apiKey:      "test-key",
		model:       "gpt-4o-mini",
		baseURL:     srvURL,
		retryConfig: fastRetry,
	}
}

// embeddingOKBody returns a minimal valid OpenAI embeddings response.
func embeddingOKBody() []byte {
	type embItem struct {
		Embedding []float64 `json:"embedding"`
	}
	type embResp struct {
		Data []embItem `json:"data"`
	}
	b, _ := json.Marshal(embResp{Data: []embItem{{Embedding: []float64{0.1, 0.2, 0.3}}}})
	return b
}

// chatOKBody returns a minimal valid OpenAI chat completion response.
func chatOKBody(content string) []byte {
	type msg struct {
		Content string `json:"content"`
	}
	type choice struct {
		Message msg `json:"message"`
	}
	type resp struct {
		Choices []choice `json:"choices"`
	}
	b, _ := json.Marshal(resp{Choices: []choice{{Message: msg{Content: content}}}})
	return b
}

// statusServer returns a handler that serves status codes from the given slice
// in order, then always uses the last entry.
func statusServer(codes []int, body func(code int) []byte) (*httptest.Server, *atomic.Int32) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1)) - 1
		if n >= len(codes) {
			n = len(codes) - 1
		}
		code := codes[n]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if body != nil {
			_, _ = w.Write(body(code))
		}
	}))
	return srv, &calls
}

// ── embedOpenAI tests ──────────────────────────────────────────────────────

func TestEmbedOpenAI_RetryOn429(t *testing.T) {
	srv, calls := statusServer(
		[]int{429, 429, 200},
		func(code int) []byte {
			if code == 200 {
				return embeddingOKBody()
			}
			return []byte(`{"error":{"message":"rate limited"}}`)
		},
	)
	defer srv.Close()

	e := newTestEmbedder(srv.URL)
	emb, err := e.embedOpenAI(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emb) == 0 {
		t.Fatal("expected non-empty embedding")
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 server calls, got %d", got)
	}
}

func TestEmbedOpenAI_RetryOn500(t *testing.T) {
	srv, calls := statusServer(
		[]int{500, 200},
		func(code int) []byte {
			if code == 200 {
				return embeddingOKBody()
			}
			return []byte(`{"error":{"message":"internal"}}`)
		},
	)
	defer srv.Close()

	e := newTestEmbedder(srv.URL)
	_, err := e.embedOpenAI(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 server calls, got %d", got)
	}
}

func TestEmbedOpenAI_NoRetryOn400(t *testing.T) {
	srv, calls := statusServer(
		[]int{400},
		func(_ int) []byte { return []byte(`{"error":{"message":"bad request"}}`) },
	)
	defer srv.Close()

	e := newTestEmbedder(srv.URL)
	_, err := e.embedOpenAI(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 server call, got %d", got)
	}
}

func TestEmbedOpenAI_ExhaustsRetries(t *testing.T) {
	srv, calls := statusServer(
		[]int{429},
		func(_ int) []byte { return []byte(`{"error":{"message":"still rate limited"}}`) },
	)
	defer srv.Close()

	e := newTestEmbedder(srv.URL)
	_, err := e.embedOpenAI(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	// MaxRetries=2 → initial attempt + 2 retries = 3 calls
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 server calls (1 + 2 retries), got %d", got)
	}
}

// ── generateOpenAIText tests ───────────────────────────────────────────────

func TestGenerateOpenAIText_RetryOn429(t *testing.T) {
	srv, calls := statusServer(
		[]int{429, 200},
		func(code int) []byte {
			if code == 200 {
				return chatOKBody("pong")
			}
			return []byte(`{"error":{"message":"rate limited"}}`)
		},
	)
	defer srv.Close()

	l := newTestLLMClient(srv.URL)
	text, err := l.generateOpenAIText(context.Background(), "ping", 0.0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "pong" {
		t.Fatalf("expected 'pong', got %q", text)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 server calls, got %d", got)
	}
}

func TestGenerateOpenAIText_RetryOn500(t *testing.T) {
	srv, calls := statusServer(
		[]int{500, 200},
		func(code int) []byte {
			if code == 200 {
				return chatOKBody("ok")
			}
			return []byte(`{"error":{"message":"internal"}}`)
		},
	)
	defer srv.Close()

	l := newTestLLMClient(srv.URL)
	_, err := l.generateOpenAIText(context.Background(), "ping", 0.0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 server calls, got %d", got)
	}
}

func TestGenerateOpenAIText_NoRetryOn400(t *testing.T) {
	srv, calls := statusServer(
		[]int{400},
		func(_ int) []byte { return []byte(`{"error":{"message":"bad request"}}`) },
	)
	defer srv.Close()

	l := newTestLLMClient(srv.URL)
	_, err := l.generateOpenAIText(context.Background(), "ping", 0.0, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 server call, got %d", got)
	}
}

func TestGenerateOpenAIText_ExhaustsRetries(t *testing.T) {
	srv, calls := statusServer(
		[]int{429},
		func(_ int) []byte { return []byte(`{"error":{"message":"still rate limited"}}`) },
	)
	defer srv.Close()

	l := newTestLLMClient(srv.URL)
	_, err := l.generateOpenAIText(context.Background(), "ping", 0.0, false)
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 server calls (1 + 2 retries), got %d", got)
	}
}
