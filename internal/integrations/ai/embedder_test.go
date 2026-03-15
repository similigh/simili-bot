// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-03-05
// Last Modified: 2026-03-05

package ai

import (
	"strings"
	"testing"
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
