// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-18

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

	// LLM configures the LLM provider.
	LLM LLMConfig `yaml:"llm"`

	// Workflow is a preset workflow name (e.g., "issue-triage").
	Workflow string `yaml:"workflow,omitempty"`

	// Steps is a custom list of pipeline steps (overrides workflow).
	Steps []string `yaml:"steps,omitempty"`

	// Defaults contains default behavior settings.
	Defaults DefaultsConfig `yaml:"defaults"`

	// Repositories lists the repositories this config applies to.
	Repositories []RepositoryConfig `yaml:"repositories,omitempty"`

	// Transfer configures cross-repository issue routing.
	Transfer TransferConfig `yaml:"transfer,omitempty"`

	// AutoClose configures the automatic closure of confirmed duplicate issues.
	AutoClose AutoCloseConfig `yaml:"auto_close,omitempty"`

	// BotUsers is a list of GitHub usernames whose events should be ignored
	// to prevent infinite comment loops. Built-in heuristics (e.g. "[bot]" suffix,
	// "gh-simili" prefix) always apply in addition to this list.
	BotUsers []string `yaml:"bot_users,omitempty"`
}

// AutoCloseConfig configures the auto-close behavior for duplicate issues.
type AutoCloseConfig struct {
	GracePeriodHours           int  `yaml:"grace_period_hours"` // Hours after labeling before auto-close (default: 72)
	GracePeriodMinutesOverride int  `yaml:"-"`                  // CLI-only override in minutes (for testing; 0 = use GracePeriodHours)
	DryRun                     bool `yaml:"dry_run,omitempty"`  // If true, log actions without executing
}

// QdrantConfig holds Qdrant connection settings.
type QdrantConfig struct {
	URL        string `yaml:"url"`
	APIKey     string `yaml:"api_key"`
	Collection string `yaml:"collection"`
}

// EmbeddingConfig holds embedding provider settings.
type EmbeddingConfig struct {
	Provider   string `yaml:"provider"`
	APIKey     string `yaml:"api_key"`
	Model      string `yaml:"model,omitempty"`
	Dimensions int    `yaml:"dimensions,omitempty"`
}

// LLMConfig holds LLM provider settings.
type LLMConfig struct {
	Provider    string   `yaml:"provider"`
	APIKey      string   `yaml:"api_key"`
	Model       string   `yaml:"model,omitempty"`
	Temperature *float64 `yaml:"temperature,omitempty"`
}

// DefaultsConfig holds default behavior settings.
type DefaultsConfig struct {
	SimilarityThreshold float64 `yaml:"similarity_threshold"`
	MaxSimilarToShow    int     `yaml:"max_similar_to_show"`
	CrossRepoSearch     *bool   `yaml:"cross_repo_search,omitempty"`
}

// RepositoryConfig defines a repository and its settings.
type RepositoryConfig struct {
	Org         string   `yaml:"org"`
	Repo        string   `yaml:"repo"`
	Description string   `yaml:"description,omitempty"` // For LLM routing
	Labels      []string `yaml:"labels,omitempty"`
	Enabled     bool     `yaml:"enabled"`
}

// TransferRule defines a rule for transferring issues to another repository.
type TransferRule struct {
	Name          string   `yaml:"name"`
	Priority      int      `yaml:"priority,omitempty"`
	Target        string   `yaml:"target"`               // "owner/repo"
	Labels        []string `yaml:"labels,omitempty"`     // ALL must match
	LabelsAny     []string `yaml:"labels_any,omitempty"` // ANY must match
	TitleContains []string `yaml:"title_contains,omitempty"`
	BodyContains  []string `yaml:"body_contains,omitempty"`
	Author        []string `yaml:"author,omitempty"`
	Enabled       *bool    `yaml:"enabled,omitempty"`
}

// TransferConfig holds transfer routing settings.
type TransferConfig struct {
	Enabled                      *bool          `yaml:"enabled,omitempty"`
	Rules                        []TransferRule `yaml:"rules,omitempty"`
	LLMRoutingEnabled            *bool          `yaml:"llm_routing_enabled,omitempty"`
	HighConfidence               float64        `yaml:"high_confidence,omitempty"`                // Default: 0.9
	MediumConfidence             float64        `yaml:"medium_confidence,omitempty"`              // Default: 0.6
	DuplicateConfidenceThreshold float64        `yaml:"duplicate_confidence_threshold,omitempty"` // Default: 0.8
	RepoCollection               string         `yaml:"repo_collection,omitempty"`                // Collection for repository documentation
}

// Load reads a config file from the given path and expands environment variables.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg, err := parseRaw(data)
	if err != nil {
		return nil, err
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadWithInheritance loads a config and resolves the 'extends' chain.
// The fetcher function is used to retrieve remote configs.
func LoadWithInheritance(path string, fetcher func(ref string) ([]byte, error)) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg, err := parseRaw(data)
	if err != nil {
		return nil, err
	}

	if cfg.Extends == "" {
		cfg.applyDefaults()
		if err := cfg.Validate(); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	// Fetch and parse the parent config
	parentData, err := fetcher(cfg.Extends)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch parent config '%s': %w", cfg.Extends, err)
	}

	parentCfg, err := parseRaw(parentData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parent config: %w", err)
	}

	// Merge: child overrides parent
	merged := mergeConfigs(parentCfg, cfg)
	merged.applyDefaults()
	if err := merged.Validate(); err != nil {
		return nil, err
	}

	return merged, nil
}

// parseRaw parses YAML content and expands environment variables without applying defaults.
func parseRaw(data []byte) (*Config, error) {
	// Expand environment variables in the YAML content
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// Validate ensures required configuration fields are present.
func (c *Config) Validate() error {
	requiredFields := []struct {
		name   string
		envVar string
		value  string
	}{
		{name: "qdrant.url", envVar: "QDRANT_URL", value: c.Qdrant.URL},
		{name: "qdrant.api_key", envVar: "QDRANT_API_KEY", value: c.Qdrant.APIKey},
		{name: "qdrant.collection", envVar: "QDRANT_COLLECTION", value: c.Qdrant.Collection},
		{name: "embedding.api_key", envVar: "EMBEDDING_API_KEY", value: c.Embedding.APIKey},
		{name: "llm.api_key", envVar: "LLM_API_KEY", value: c.LLM.APIKey},
	}

	for _, field := range requiredFields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf(
				"config validation failed: %s is empty (check %s environment variable)",
				field.name,
				field.envVar,
			)
		}
	}

	return nil
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
	if c.Defaults.CrossRepoSearch == nil {
		t := true
		c.Defaults.CrossRepoSearch = &t
	}
	if c.Embedding.Provider == "" {
		c.Embedding.Provider = "gemini"
	}
	if c.Embedding.Dimensions == 0 {
		c.Embedding.Dimensions = 768
	}
	if c.LLM.Provider == "" {
		c.LLM.Provider = "gemini"
	}
	if c.LLM.Model == "" {
		c.LLM.Model = "gemini-2.5-flash"
	}
	// Transfer defaults
	if c.Transfer.Enabled == nil {
		f := false
		c.Transfer.Enabled = &f
	}
	if c.Transfer.LLMRoutingEnabled == nil {
		f := false
		c.Transfer.LLMRoutingEnabled = &f
	}
	if c.Transfer.HighConfidence == 0 {
		c.Transfer.HighConfidence = 0.9
	}
	if c.Transfer.MediumConfidence == 0 {
		c.Transfer.MediumConfidence = 0.6
	}
	if c.Transfer.DuplicateConfidenceThreshold == 0 {
		c.Transfer.DuplicateConfidenceThreshold = 0.8
	}
	if c.Transfer.RepoCollection == "" {
		c.Transfer.RepoCollection = "simili_repos"
	}
	// Auto-close defaults
	if c.AutoClose.GracePeriodHours == 0 {
		c.AutoClose.GracePeriodHours = 72
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

	// LLM: override if any field is set
	if child.LLM.Provider != "" {
		result.LLM.Provider = child.LLM.Provider
	}
	if child.LLM.APIKey != "" {
		result.LLM.APIKey = child.LLM.APIKey
	}
	if child.LLM.Model != "" {
		result.LLM.Model = child.LLM.Model
	}
	if child.LLM.Temperature != nil {
		result.LLM.Temperature = child.LLM.Temperature
	}

	// Defaults: override if non-zero
	if child.Defaults.SimilarityThreshold != 0 {
		result.Defaults.SimilarityThreshold = child.Defaults.SimilarityThreshold
	}
	if child.Defaults.MaxSimilarToShow != 0 {
		result.Defaults.MaxSimilarToShow = child.Defaults.MaxSimilarToShow
	}
	if child.Defaults.CrossRepoSearch != nil {
		result.Defaults.CrossRepoSearch = child.Defaults.CrossRepoSearch
	}

	// Repositories: child completely overrides if non-empty
	if len(child.Repositories) > 0 {
		result.Repositories = child.Repositories
	}

	// Transfer: override if fields are set
	if child.Transfer.Enabled != nil {
		result.Transfer.Enabled = child.Transfer.Enabled
	}
	if child.Transfer.LLMRoutingEnabled != nil {
		result.Transfer.LLMRoutingEnabled = child.Transfer.LLMRoutingEnabled
	}
	if len(child.Transfer.Rules) > 0 {
		result.Transfer.Rules = child.Transfer.Rules
	}
	if child.Transfer.HighConfidence != 0 {
		result.Transfer.HighConfidence = child.Transfer.HighConfidence
	}
	if child.Transfer.MediumConfidence != 0 {
		result.Transfer.MediumConfidence = child.Transfer.MediumConfidence
	}
	if child.Transfer.DuplicateConfidenceThreshold != 0 {
		result.Transfer.DuplicateConfidenceThreshold = child.Transfer.DuplicateConfidenceThreshold
	}
	if child.Transfer.RepoCollection != "" {
		result.Transfer.RepoCollection = child.Transfer.RepoCollection
	}

	// AutoClose: override if fields are set
	if child.AutoClose.GracePeriodHours != 0 {
		result.AutoClose.GracePeriodHours = child.AutoClose.GracePeriodHours
	}
	if child.AutoClose.DryRun {
		result.AutoClose.DryRun = child.AutoClose.DryRun
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
