package gemini

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"rate limit 429", errors.New("googleapi: Error 429: Resource exhausted"), true},
		{"ResourceExhausted gRPC", errors.New("rpc error: code = ResourceExhausted"), true},
		{"server error 500", errors.New("googleapi: Error 500: Internal Server Error"), true},
		{"bad gateway 502", errors.New("HTTP 502: Bad Gateway"), true},
		{"unavailable 503", errors.New("googleapi: Error 503: Service Unavailable"), true},
		{"gateway timeout 504", errors.New("HTTP 504: Gateway Timeout"), true},
		{"Unavailable keyword", errors.New("rpc error: code = Unavailable"), true},
		{"Internal keyword", errors.New("rpc error: code = Internal"), true},
		{"client error 400", errors.New("googleapi: Error 400: Bad Request"), false},
		{"forbidden 403", errors.New("googleapi: Error 403: Forbidden"), false},
		{"not found 404", errors.New("googleapi: Error 404: Not Found"), false},
		{"generic error", errors.New("something went wrong"), false},
		{"empty text error", errors.New("text cannot be empty"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableError(tt.err)
			if got != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestWithRetry_Success(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond, JitterRatio: 0}

	calls := 0
	result, err := withRetry(context.Background(), cfg, "test", func() (string, error) {
		calls++
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("got %q, want %q", result, "ok")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestWithRetry_SuccessAfterRetries(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond, JitterRatio: 0}

	calls := 0
	result, err := withRetry(context.Background(), cfg, "test", func() (string, error) {
		calls++
		if calls < 3 {
			return "", errors.New("googleapi: Error 429: Rate limited")
		}
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("got %q, want %q", result, "ok")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond, JitterRatio: 0}

	calls := 0
	_, err := withRetry(context.Background(), cfg, "test", func() (string, error) {
		calls++
		return "", errors.New("googleapi: Error 400: Bad Request")
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry for 400), got %d", calls)
	}
}

func TestWithRetry_ExhaustedRetries(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 2, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond, JitterRatio: 0}

	calls := 0
	_, err := withRetry(context.Background(), cfg, "test-op", func() (string, error) {
		calls++
		return "", errors.New("googleapi: Error 503: Service Unavailable")
	})

	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	// 1 initial + 2 retries = 3
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestWithRetry_ContextCancelled(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 5, BaseDelay: 100 * time.Millisecond, MaxDelay: 1 * time.Second, JitterRatio: 0}

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := withRetry(ctx, cfg, "test", func() (string, error) {
		calls++
		return "", errors.New("googleapi: Error 429: Rate limited")
	})

	if err == nil {
		t.Fatal("expected error on context cancel")
	}
}
