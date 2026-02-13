// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-05

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
	Body       string // Full text content from vector DB
	URL        string
	Similarity float64
	State      string
}

// TriageResult holds the result of issue triage analysis.
type TriageResult struct {
	Quality         string   `json:"quality"` // "good", "needs-improvement", "poor"
	SuggestedLabels []string `json:"suggested_labels"`
	Reasoning       string   `json:"reasoning"`
	IsDuplicate     bool     `json:"is_duplicate"`
	DuplicateReason string   `json:"duplicate_reason"`
}

// RouteIssueInput represents input for repository routing.
type RouteIssueInput struct {
	Issue        *IssueInput
	Repositories []RepositoryCandidate
	CurrentRepo  string // Current repository (org/repo) where issue was created
}

// RepositoryCandidate represents a repository option for routing.
type RepositoryCandidate struct {
	Org         string
	Repo        string
	Description string
	Definition  string // Full repository documentation (README, etc.)
}

// RouterResult holds repository routing analysis.
type RouterResult struct {
	Rankings  []RepositoryRanking
	BestMatch *RepositoryRanking
}

// RepositoryRanking represents a repository match with confidence.
type RepositoryRanking struct {
	Org        string  `json:"org"`
	Repo       string  `json:"repo"`
	Confidence float64 `json:"confidence"` // 0.0-1.0
	Reasoning  string  `json:"reasoning"`
}

// QualityResult holds issue quality assessment.
type QualityResult struct {
	Score       float64  `json:"score"`       // 0.0 (poor) to 1.0 (excellent)
	Assessment  string   `json:"assessment"`  // "excellent"|"good"|"needs-improvement"|"poor"
	Issues      []string `json:"issues"`      // Missing elements
	Suggestions []string `json:"suggestions"` // How to improve
	Reasoning   string   `json:"reasoning"`
}

// DuplicateCheckInput represents input for duplicate detection.
type DuplicateCheckInput struct {
	CurrentIssue  *IssueInput
	SimilarIssues []SimilarIssueInput
}

// DuplicateResult holds duplicate detection analysis.
type DuplicateResult struct {
	IsDuplicate   bool            `json:"is_duplicate"`
	DuplicateOf   int             `json:"duplicate_of"` // Issue number
	Confidence    float64         `json:"confidence"`   // 0.0-1.0
	Reasoning     string          `json:"reasoning"`
	SimilarIssues json.RawMessage `json:"similar_issues"` // Flexible: can be []int or []object
}

// NewLLMClient creates a new Gemini LLM client.
func NewLLMClient(apiKey, model string) (*LLMClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	if model == "" {
		model = "gemini-2.0-flash-lite" // Fast and cost-effective
	}

	return &LLMClient{
		client: client,
		model:  model,
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

// RouteIssue analyzes issue intent and ranks repositories by relevance.
func (l *LLMClient) RouteIssue(ctx context.Context, input *RouteIssueInput) (*RouterResult, error) {
	if len(input.Repositories) == 0 {
		return &RouterResult{Rankings: []RepositoryRanking{}, BestMatch: nil}, nil
	}

	prompt := buildRouteIssuePrompt(input)

	model := l.client.GenerativeModel(l.model)
	model.SetTemperature(0.3)
	model.ResponseMIMEType = "application/json"

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to route issue: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from LLM")
	}

	var responseText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			responseText += string(txt)
		}
	}

	var result struct {
		Rankings []RepositoryRanking `json:"rankings"`
	}
	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		return nil, fmt.Errorf("failed to parse routing response: %w", err)
	}

	// Ensure non-nil slices
	if result.Rankings == nil {
		result.Rankings = []RepositoryRanking{}
	}

	// Find best match (highest confidence)
	var bestMatch *RepositoryRanking
	for i := range result.Rankings {
		if bestMatch == nil || result.Rankings[i].Confidence > bestMatch.Confidence {
			bestMatch = &result.Rankings[i]
		}
	}

	return &RouterResult{
		Rankings:  result.Rankings,
		BestMatch: bestMatch,
	}, nil
}

// AssessQuality evaluates issue completeness and clarity.
func (l *LLMClient) AssessQuality(ctx context.Context, issue *IssueInput) (*QualityResult, error) {
	prompt := buildQualityAssessmentPrompt(issue)

	model := l.client.GenerativeModel(l.model)
	model.SetTemperature(0.3)
	model.ResponseMIMEType = "application/json"

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to assess quality: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from LLM")
	}

	var responseText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			responseText += string(txt)
		}
	}

	var result QualityResult
	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		return nil, fmt.Errorf("failed to parse quality response: %w", err)
	}

	// Ensure non-nil slices
	if result.Issues == nil {
		result.Issues = []string{}
	}
	if result.Suggestions == nil {
		result.Suggestions = []string{}
	}

	// Normalize assessment
	switch strings.ToLower(result.Assessment) {
	case "excellent", "good", "needs-improvement", "poor":
		result.Assessment = strings.ToLower(result.Assessment)
	default:
		result.Assessment = "good"
	}

	return &result, nil
}

// DetectDuplicate analyzes semantic similarity for duplicate detection.
func (l *LLMClient) DetectDuplicate(ctx context.Context, input *DuplicateCheckInput) (*DuplicateResult, error) {
	if len(input.SimilarIssues) == 0 {
		return &DuplicateResult{IsDuplicate: false, SimilarIssues: json.RawMessage("[]")}, nil
	}

	prompt := buildDuplicateDetectionPrompt(input)

	model := l.client.GenerativeModel(l.model)
	model.SetTemperature(0.3)
	model.ResponseMIMEType = "application/json"

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to detect duplicate: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from LLM")
	}

	var responseText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			responseText += string(txt)
		}
	}

	var result DuplicateResult
	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		return nil, fmt.Errorf("failed to parse duplicate response: %w", err)
	}

	// Ensure non-nil SimilarIssues
	if result.SimilarIssues == nil {
		result.SimilarIssues = json.RawMessage("[]")
	}

	return &result, nil
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
