// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-18

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestConfigDefaults verifies that default values are applied correctly.
func TestConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()

	if cfg.Defaults.SimilarityThreshold != 0.65 {
		t.Errorf("Expected SimilarityThreshold to be 0.65, got %f", cfg.Defaults.SimilarityThreshold)
	}

	if cfg.Defaults.MaxSimilarToShow != 5 {
		t.Errorf("Expected MaxSimilarToShow to be 5, got %d", cfg.Defaults.MaxSimilarToShow)
	}

	if cfg.Embedding.Provider != "gemini" {
		t.Errorf("Expected Embedding.Provider to be 'gemini', got %s", cfg.Embedding.Provider)
	}
}

func TestLLMConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()

	if cfg.LLM.Provider != "gemini" {
		t.Errorf("Expected LLM.Provider to be 'gemini', got %s", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "gemini-2.5-flash" {
		t.Errorf("Expected LLM.Model to be 'gemini-2.5-flash', got %s", cfg.LLM.Model)
	}
}

func TestMergeConfigsLLM(t *testing.T) {
	parent := &Config{}
	parent.applyDefaults()

	child := &Config{
		LLM: LLMConfig{
			Model: "gemini-2.0-flash",
		},
	}

	merged := mergeConfigs(parent, child)
	if merged.LLM.Model != "gemini-2.0-flash" {
		t.Errorf("Expected merged LLM.Model to be 'gemini-2.0-flash', got %s", merged.LLM.Model)
	}
	if merged.LLM.Provider != "gemini" {
		t.Errorf("Expected merged LLM.Provider to be 'gemini', got %s", merged.LLM.Provider)
	}
}

func TestLoadConfigWithLLM(t *testing.T) {
	yamlContent := `
qdrant:
  url: "http://localhost:6334"
  collection: "test"
embedding:
  provider: gemini
llm:
  provider: gemini
  model: gemini-2.5-flash
defaults:
  similarity_threshold: 0.7
`
	cfg, err := parseRaw([]byte(yamlContent))
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}
	if cfg.LLM.Model != "gemini-2.5-flash" {
		t.Errorf("Expected LLM.Model 'gemini-2.5-flash', got '%s'", cfg.LLM.Model)
	}
	if cfg.LLM.Provider != "gemini" {
		t.Errorf("Expected LLM.Provider 'gemini', got '%s'", cfg.LLM.Provider)
	}
}

// TestParseExtendsRef verifies extends reference parsing.
func TestParseExtendsRef(t *testing.T) {
	tests := []struct {
		name        string
		ref         string
		wantOrg     string
		wantRepo    string
		wantBranch  string
		wantPath    string
		expectError bool
	}{
		{
			name:       "valid ref with default path",
			ref:        "org/repo@main",
			wantOrg:    "org",
			wantRepo:   "repo",
			wantBranch: "main",
			wantPath:   ".github/simili.yaml",
		},
		{
			name:       "valid ref with custom path",
			ref:        "org/repo@main:custom/path.yaml",
			wantOrg:    "org",
			wantRepo:   "repo",
			wantBranch: "main",
			wantPath:   "custom/path.yaml",
		},
		{
			name:        "invalid ref missing branch",
			ref:         "org/repo",
			expectError: true,
		},
		{
			name:        "invalid ref missing repo",
			ref:         "org@main",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, repo, branch, path, err := ParseExtendsRef(tt.ref)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for ref %s, got nil", tt.ref)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if org != tt.wantOrg {
				t.Errorf("Expected org %s, got %s", tt.wantOrg, org)
			}
			if repo != tt.wantRepo {
				t.Errorf("Expected repo %s, got %s", tt.wantRepo, repo)
			}
			if branch != tt.wantBranch {
				t.Errorf("Expected branch %s, got %s", tt.wantBranch, branch)
			}
			if path != tt.wantPath {
				t.Errorf("Expected path %s, got %s", tt.wantPath, path)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	baseConfig := Config{
		Qdrant: QdrantConfig{
			URL:        "https://example.qdrant.io:6334",
			APIKey:     "qdrant-key",
			Collection: "issues",
		},
		Embedding: EmbeddingConfig{
			APIKey: "embedding-key",
		},
		LLM: LLMConfig{
			APIKey: "llm-key",
		},
	}

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "all valid",
			cfg:  baseConfig,
		},
		{
			name: "missing qdrant url",
			cfg: func() Config {
				cfg := baseConfig
				cfg.Qdrant.URL = ""
				return cfg
			}(),
			wantErr: "config validation failed: qdrant.url is empty (check QDRANT_URL environment variable)",
		},
		{
			name: "missing qdrant api key",
			cfg: func() Config {
				cfg := baseConfig
				cfg.Qdrant.APIKey = ""
				return cfg
			}(),
			wantErr: "config validation failed: qdrant.api_key is empty (check QDRANT_API_KEY environment variable)",
		},
		{
			name: "missing qdrant collection",
			cfg: func() Config {
				cfg := baseConfig
				cfg.Qdrant.Collection = ""
				return cfg
			}(),
			wantErr: "config validation failed: qdrant.collection is empty (check QDRANT_COLLECTION environment variable)",
		},
		{
			name: "missing embedding api key",
			cfg: func() Config {
				cfg := baseConfig
				cfg.Embedding.APIKey = ""
				return cfg
			}(),
			wantErr: "config validation failed: embedding.api_key is empty (check EMBEDDING_API_KEY environment variable)",
		},
		{
			name: "missing llm api key",
			cfg: func() Config {
				cfg := baseConfig
				cfg.LLM.APIKey = ""
				return cfg
			}(),
			wantErr: "config validation failed: llm.api_key is empty (check LLM_API_KEY environment variable)",
		},
		{
			name: "partial config",
			cfg: Config{
				Qdrant: QdrantConfig{
					URL: "https://example.qdrant.io:6334",
				},
			},
			wantErr: "config validation failed: qdrant.api_key is empty (check QDRANT_API_KEY environment variable)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" && err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("Expected error %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestLoadValidatesExpandedEnvironmentVariables(t *testing.T) {
	t.Setenv("QDRANT_URL", "")
	t.Setenv("QDRANT_API_KEY", "qdrant-key")
	t.Setenv("QDRANT_COLLECTION", "issues")
	t.Setenv("EMBEDDING_API_KEY", "embedding-key")
	t.Setenv("LLM_API_KEY", "llm-key")

	yamlContent := `qdrant:
  url: "${QDRANT_URL}"
  api_key: "${QDRANT_API_KEY}"
  collection: "${QDRANT_COLLECTION}"
embedding:
  api_key: "${EMBEDDING_API_KEY}"
llm:
  api_key: "${LLM_API_KEY}"
`

	path := filepath.Join(t.TempDir(), "simili.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatalf("Expected validation error, got nil")
	}

	wantErr := "config validation failed: qdrant.url is empty (check QDRANT_URL environment variable)"
	if err.Error() != wantErr {
		t.Fatalf("Expected error %q, got %q", wantErr, err.Error())
	}
}

func TestLoadWithInheritanceValidatesMergedConfig(t *testing.T) {
	childContent := `extends: "org/repo@main"
`

	parentContent := `qdrant:
  url: "https://example.qdrant.io:6334"
  api_key: "qdrant-key"
  collection: "issues"
embedding:
  api_key: "embedding-key"
llm:
  api_key: ""
`

	path := filepath.Join(t.TempDir(), "simili.yaml")
	if err := os.WriteFile(path, []byte(childContent), 0644); err != nil {
		t.Fatalf("Failed to write child config: %v", err)
	}

	_, err := LoadWithInheritance(path, func(ref string) ([]byte, error) {
		if ref != "org/repo@main" {
			return nil, fmt.Errorf("unexpected ref: %s", ref)
		}
		return []byte(parentContent), nil
	})
	if err == nil {
		t.Fatalf("Expected validation error, got nil")
	}

	wantErr := "config validation failed: llm.api_key is empty (check LLM_API_KEY environment variable)"
	if err.Error() != wantErr {
		t.Fatalf("Expected error %q, got %q", wantErr, err.Error())
	}
}
