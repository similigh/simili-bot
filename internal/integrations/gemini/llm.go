// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// LLMClient provides LLM-based analysis using Gemini.
type LLMClient struct {
	client *genai.Client
	model  string
}

// IssueInput represents the issue data needed for analysis.
type IssueInput struct {
	Title  string
	Body   string
	Author string
	Labels []string
}

// SimilarIssueInput represents a similar issue found.
type SimilarIssueInput struct {
	Number     int
	Title      string
	URL        string
	Similarity float64
	State      string
}

// TriageResult holds the result of issue triage analysis.
type TriageResult struct {
	Quality         string   `json:"quality"`          // "good", "needs-improvement", "poor"
	SuggestedLabels []string `json:"suggested_labels"`
	Reasoning       string   `json:"reasoning"`
	IsDuplicate     bool     `json:"is_duplicate"`
	DuplicateReason string   `json:"duplicate_reason"`
}

// NewLLMClient creates a new Gemini LLM client.
func NewLLMClient(apiKey string) (*LLMClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &LLMClient{
		client: client,
		model:  "gemini-2.0-flash-lite", // Fast and cost-effective
	}, nil
}

// Close closes the Gemini client.
func (l *LLMClient) Close() error {
	return l.client.Close()
}

// AnalyzeIssue performs triage analysis on an issue.
func (l *LLMClient) AnalyzeIssue(ctx context.Context, issue *IssueInput) (*TriageResult, error) {
	prompt := buildTriagePromptJSON(issue)

	model := l.client.GenerativeModel(l.model)
	model.SetTemperature(0.3) // Lower temperature for more consistent results
	// Request JSON response for structured parsing
	model.ResponseMIMEType = "application/json"

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to analyze issue: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from LLM")
	}

	// Extract text from response
	var responseText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			responseText += string(txt)
		}
	}

	// Parse JSON response into TriageResult
	result, err := parseTriageResponseJSON(responseText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}
	return result, nil
}

// GenerateResponse creates a comment for similar issues.
func (l *LLMClient) GenerateResponse(ctx context.Context, similar []SimilarIssueInput) (string, error) {
	if len(similar) == 0 {
		return "", nil
	}

	prompt := buildResponsePrompt(similar)

	model := l.client.GenerativeModel(l.model)
	model.SetTemperature(0.5) // Slightly higher for more natural language

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	// Extract text from response
	var responseText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			responseText += string(txt)
		}
	}

	return strings.TrimSpace(responseText), nil
}

// parseTriageResponseJSON parses the JSON LLM response into a TriageResult.
func parseTriageResponseJSON(response string) (*TriageResult, error) {
	var result TriageResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// If JSON parsing fails, fall back to legacy string parsing
		return parseTriageResponseLegacy(response), nil
	}

	// Validate and normalize quality
	switch strings.ToLower(result.Quality) {
	case "good", "needs-improvement", "poor":
		result.Quality = strings.ToLower(result.Quality)
	default:
		result.Quality = "good" // Default
	}

	// Ensure non-nil slices
	if result.SuggestedLabels == nil {
		result.SuggestedLabels = []string{}
	}

	return &result, nil
}

// parseTriageResponseLegacy is a fallback parser for non-JSON responses.
// Deprecated: Use JSON structured output instead.
func parseTriageResponseLegacy(response string) *TriageResult {
	result := &TriageResult{
		Quality:         "good", // Default
		SuggestedLabels: []string{},
		Reasoning:       response,
	}

	lower := strings.ToLower(response)

	// Parse quality
	if strings.Contains(lower, "poor quality") || strings.Contains(lower, "quality: poor") {
		result.Quality = "poor"
	} else if strings.Contains(lower, "needs improvement") || strings.Contains(lower, "quality: needs-improvement") {
		result.Quality = "needs-improvement"
	}

	// Parse labels (look for common patterns)
	labels := []string{}
	if strings.Contains(lower, "bug") {
		labels = append(labels, "bug")
	}
	if strings.Contains(lower, "feature") || strings.Contains(lower, "enhancement") {
		labels = append(labels, "enhancement")
	}
	if strings.Contains(lower, "documentation") || strings.Contains(lower, "docs") {
		labels = append(labels, "documentation")
	}
	if strings.Contains(lower, "question") {
		labels = append(labels, "question")
	}
	result.SuggestedLabels = labels

	// Parse duplicate status
	if strings.Contains(lower, "duplicate") || strings.Contains(lower, "similar to") {
		result.IsDuplicate = true
		result.DuplicateReason = "LLM detected potential duplicate"
	}

	return result
}
