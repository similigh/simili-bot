// Package gemini provides Gemini AI integration for embeddings and LLM.
package gemini

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
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

// isRetryableError checks whether an error from the Gemini API is transient
// and should be retried. Only 429 (rate limit) and 5xx (server errors)
// are considered retryable; client errors (400, 403, 404) are not.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()

	// googleapi and Gemini SDK typically include the HTTP status code in the
	// error string, e.g. "googleapi: Error 429: ..." or "rpc error: code = ResourceExhausted".
	if strings.Contains(msg, "429") || strings.Contains(msg, "ResourceExhausted") {
		return true
	}

	// 5xx server errors
	for _, code := range []string{"500", "502", "503", "504"} {
		if strings.Contains(msg, code) {
			return true
		}
	}

	if strings.Contains(msg, "Unavailable") || strings.Contains(msg, "Internal") {
		return true
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

		// Don't retry non-transient errors
		if !isRetryableError(err) {
			return zero, err
		}

		// Exhausted retries
		if attempt == cfg.MaxRetries {
			return zero, fmt.Errorf("%s failed after %d retries: %w", operation, cfg.MaxRetries, err)
		}

		// Calculate delay: base * 2^attempt
		delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(2, float64(attempt)))

		// Add jitter to prevent thundering herd
		if cfg.JitterRatio > 0 {
			jitter := time.Duration(rand.Float64() * cfg.JitterRatio * float64(delay))
			delay += jitter
		}

		// Cap after jitter so MaxDelay is respected
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}

		// Wait or bail if context is cancelled
		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("%s: context cancelled during retry: %w", operation, ctx.Err())
		case <-time.After(delay):
			// continue to next attempt
		}
	}

	return zero, fmt.Errorf("%s: retry loop exited unexpectedly", operation)
}
