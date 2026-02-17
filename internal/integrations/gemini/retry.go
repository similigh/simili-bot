// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-15
// Last Modified: 2026-02-17

// Package gemini provides Gemini AI integration for embeddings and LLM.
package gemini

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RetryConfig holds configuration for exponential backoff retry.
type RetryConfig struct {
	MaxRetries  int           // Maximum number of retry attempts (default: 5)
	BaseDelay   time.Duration // Initial delay before first retry (default: 1s)
	MaxDelay    time.Duration // Maximum delay cap (default: 60s)
	JitterRatio float64       // Jitter as fraction of delay, 0.0-1.0 (default: 0.25)
}

// DefaultRetryConfig returns sensible defaults for Gemini API retries.
// Defaults: 5 retries, 1s base delay, 60s max delay, 25% jitter.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  5,
		BaseDelay:   1 * time.Second,
		MaxDelay:    60 * time.Second,
		JitterRatio: 0.25,
	}
}

// isRetryableError reports whether err is a transient Gemini API error that
// warrants a retry. It uses typed checking rather than string matching:
//   - REST transport errors are checked via *googleapi.Error (HTTP 429 / 5xx).
//   - gRPC transport errors are checked via gRPC status codes
//     (ResourceExhausted, Unavailable, Internal).
//
// Client errors (4xx other than 429) are not retried.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// REST transport: google.golang.org/api returns *googleapi.Error.
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == 429 || (gerr.Code >= 500 && gerr.Code < 600)
	}

	// gRPC transport: generative-ai-go can return gRPC status errors.
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.ResourceExhausted, codes.Unavailable, codes.Internal:
			return true
		}
	}

	return false
}

// withRetry executes fn with exponential backoff. It retries only on
// transient errors (429 / 5xx). Non-retryable errors are returned
// immediately so callers see them without unnecessary delay.
func withRetry[T any](ctx context.Context, cfg RetryConfig, operation string, fn func() (T, error)) (T, error) {
	var zero T

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		// Don't retry non-transient errors.
		if !isRetryableError(err) {
			return zero, err
		}

		// Exhausted retries.
		if attempt == cfg.MaxRetries {
			return zero, fmt.Errorf("%s failed after %d retries: %w", operation, cfg.MaxRetries, err)
		}

		// Calculate delay: base * 2^attempt, add jitter, then cap.
		delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(2, float64(attempt)))
		if cfg.JitterRatio > 0 {
			delay += time.Duration(rand.Float64() * cfg.JitterRatio * float64(delay))
		}
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}

		// Wait or bail if context is cancelled.
		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("%s: context cancelled during retry: %w", operation, ctx.Err())
		case <-time.After(delay):
			// continue to next attempt
		}
	}

	return zero, fmt.Errorf("%s: retry loop exited unexpectedly", operation)
}
