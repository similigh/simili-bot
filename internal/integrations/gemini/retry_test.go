// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-15
// Last Modified: 2026-02-17

package gemini

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"rate limit 429", &googleapi.Error{Code: 429, Message: "Resource exhausted"}, true},
		{"ResourceExhausted gRPC", status.New(codes.ResourceExhausted, "resource exhausted").Err(), true},
		{"server error 500", &googleapi.Error{Code: 500, Message: "Internal Server Error"}, true},
		{"bad gateway 502", &googleapi.Error{Code: 502, Message: "Bad Gateway"}, true},
		{"unavailable 503", &googleapi.Error{Code: 503, Message: "Service Unavailable"}, true},
		{"gateway timeout 504", &googleapi.Error{Code: 504, Message: "Gateway Timeout"}, true},
		{"Unavailable gRPC", status.New(codes.Unavailable, "service unavailable").Err(), true},
		{"Internal gRPC", status.New(codes.Internal, "internal error").Err(), true},
		{"client error 400", &googleapi.Error{Code: 400, Message: "Bad Request"}, false},
		{"forbidden 403", &googleapi.Error{Code: 403, Message: "Forbidden"}, false},
		{"not found 404", &googleapi.Error{Code: 404, Message: "Not Found"}, false},
		{"wrapped gRPC retryable", fmt.Errorf("embed: %w", status.New(codes.ResourceExhausted, "quota").Err()), true},
		{"wrapped gRPC non-retryable", fmt.Errorf("embed: %w", status.New(codes.NotFound, "not found").Err()), false},
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
			return "", &googleapi.Error{Code: 429, Message: "Rate limited"}
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
		return "", &googleapi.Error{Code: 400, Message: "Bad Request"}
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
		return "", &googleapi.Error{Code: 503, Message: "Service Unavailable"}
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
		return "", &googleapi.Error{Code: 429, Message: "Rate limited"}
	})

	if err == nil {
		t.Fatal("expected error on context cancel")
	}
}
