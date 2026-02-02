// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package config handles loading and merging Simili configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure.
type Config struct {
	// Extends allows inheriting from a remote config (e.g., "org/repo@branch").
	Extends string `yaml:"extends,omitempty"`

	// Qdrant configures the vector database connection.
	Qdrant QdrantConfig `yaml:"qdrant"`

	// Embedding configures the embedding provider.
	Embedding EmbeddingConfig `yaml:"embedding"`

	// Workflow is a preset workflow name (e.g., "issue-triage").
	Workflow string `yaml:"workflow,omitempty"`

	// Steps is a custom list of pipeline steps (overrides workflow).
	Steps []string `yaml:"steps,omitempty"`

	// Defaults contains default behavior settings.
	Defaults DefaultsConfig `yaml:"defaults"`

	// Repositories lists the repositories this config applies to.
	Repositories []RepositoryConfig `yaml:"repositories,omitempty"`
}

// QdrantConfig holds Qdrant connection settings.
type QdrantConfig struct {
	URL        string `yaml:"url"`
	APIKey     string `yaml:"api_key"`
	Collection string `yaml:"collection"`
}

// EmbeddingConfig holds embedding provider settings.
type EmbeddingConfig struct {
	Provider string `yaml:"provider"`
	APIKey   string `yaml:"api_key"`
	Model    string `yaml:"model,omitempty"`
}

// DefaultsConfig holds default behavior settings.
type DefaultsConfig struct {
	SimilarityThreshold float64 `yaml:"similarity_threshold"`
	MaxSimilarToShow    int     `yaml:"max_similar_to_show"`
	CrossRepoSearch     bool    `yaml:"cross_repo_search"`
}

// RepositoryConfig defines a repository and its settings.
type RepositoryConfig struct {
	Org     string   `yaml:"org"`
	Repo    string   `yaml:"repo"`
	Labels  []string `yaml:"labels,omitempty"`
	Enabled bool     `yaml:"enabled"`
}

// Load reads a config file from the given path and expands environment variables.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the YAML content
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply defaults
	cfg.applyDefaults()

	return &cfg, nil
}

// LoadWithInheritance loads a config and resolves the 'extends' chain.
// The fetcher function is used to retrieve remote configs.
func LoadWithInheritance(path string, fetcher func(ref string) ([]byte, error)) (*Config, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}

	if cfg.Extends == "" {
		return cfg, nil
	}

	// Fetch and parse the parent config
	parentData, err := fetcher(cfg.Extends)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch parent config '%s': %w", cfg.Extends, err)
	}

	expanded := os.ExpandEnv(string(parentData))
	var parentCfg Config
	if err := yaml.Unmarshal([]byte(expanded), &parentCfg); err != nil {
		return nil, fmt.Errorf("failed to parse parent config: %w", err)
	}

	// Merge: child overrides parent
	merged := mergeConfigs(&parentCfg, cfg)
	merged.applyDefaults()

	return merged, nil
}

// FindConfigPath searches for a config file in standard locations.
func FindConfigPath(explicit string) string {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
		return ""
	}

	// Search in common locations
	candidates := []string{
		".github/simili.yaml",
		".github/simili.yml",
		".simili.yaml",
		".simili.yml",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}

	return ""
}

// applyDefaults sets default values for unset fields.
func (c *Config) applyDefaults() {
	if c.Defaults.SimilarityThreshold == 0 {
		c.Defaults.SimilarityThreshold = 0.65
	}
	if c.Defaults.MaxSimilarToShow == 0 {
		c.Defaults.MaxSimilarToShow = 5
	}
	if c.Embedding.Provider == "" {
		c.Embedding.Provider = "gemini"
	}
}

// mergeConfigs merges a child config onto a parent config.
// Non-zero values in child override parent.
func mergeConfigs(parent, child *Config) *Config {
	result := *parent

	// String fields: override if non-empty
	if child.Workflow != "" {
		result.Workflow = child.Workflow
	}
	if len(child.Steps) > 0 {
		result.Steps = child.Steps
	}

	// Qdrant: override if any field is set
	if child.Qdrant.URL != "" {
		result.Qdrant.URL = child.Qdrant.URL
	}
	if child.Qdrant.APIKey != "" {
		result.Qdrant.APIKey = child.Qdrant.APIKey
	}
	if child.Qdrant.Collection != "" {
		result.Qdrant.Collection = child.Qdrant.Collection
	}

	// Embedding: override if any field is set
	if child.Embedding.Provider != "" {
		result.Embedding.Provider = child.Embedding.Provider
	}
	if child.Embedding.APIKey != "" {
		result.Embedding.APIKey = child.Embedding.APIKey
	}
	if child.Embedding.Model != "" {
		result.Embedding.Model = child.Embedding.Model
	}

	// Defaults: override if non-zero
	if child.Defaults.SimilarityThreshold != 0 {
		result.Defaults.SimilarityThreshold = child.Defaults.SimilarityThreshold
	}
	if child.Defaults.MaxSimilarToShow != 0 {
		result.Defaults.MaxSimilarToShow = child.Defaults.MaxSimilarToShow
	}
	// CrossRepoSearch: always take the child value so it can override parent true -> false and vice versa
	result.Defaults.CrossRepoSearch = child.Defaults.CrossRepoSearch

	// Repositories: child completely overrides if non-empty
	if len(child.Repositories) > 0 {
		result.Repositories = child.Repositories
	}

	return &result
}

// ParseExtendsRef parses "org/repo@branch" into components.
func ParseExtendsRef(ref string) (org, repo, branch, path string, err error) {
	// Format: org/repo@branch or org/repo@branch:path
	parts := strings.SplitN(ref, "@", 2)
	if len(parts) != 2 {
		return "", "", "", "", fmt.Errorf("invalid extends reference: %s (expected org/repo@branch)", ref)
	}

	orgRepo := strings.SplitN(parts[0], "/", 2)
	if len(orgRepo) != 2 {
		return "", "", "", "", fmt.Errorf("invalid extends reference: %s (expected org/repo)", ref)
	}

	org = orgRepo[0]
	repo = orgRepo[1]

	// Check for path
	branchPath := strings.SplitN(parts[1], ":", 2)
	branch = branchPath[0]
	if len(branchPath) == 2 {
		path = branchPath[1]
	} else {
		path = ".github/simili.yaml" // default path
	}

	return org, repo, branch, path, nil
}
