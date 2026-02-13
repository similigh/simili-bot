// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-12

package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// LLMClient provides LLM-based analysis using Gemini or OpenAI.
type LLMClient struct {
	provider Provider
	gemini   *genai.Client
	openAI   *http.Client
	apiKey   string
	model    string
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

// PRDuplicateCandidateInput represents an issue/PR candidate for PR duplicate checking.
type PRDuplicateCandidateInput struct {
	ID         string
	EntityType string // "issue" or "pull_request"
	Org        string
	Repo       string
	Number     int
	Title      string
	Body       string
	URL        string
	Similarity float64
	State      string
}

// PRDuplicateCheckInput represents input for PR duplicate detection.
type PRDuplicateCheckInput struct {
	PullRequest *IssueInput
	Candidates  []PRDuplicateCandidateInput
}

// PRDuplicateResult holds duplicate detection result for pull requests.
type PRDuplicateResult struct {
	IsDuplicate bool    `json:"is_duplicate"`
	DuplicateID string  `json:"duplicate_id"` // Candidate ID, e.g. issue:org/repo#123
	Confidence  float64 `json:"confidence"`   // 0.0-1.0
	Reasoning   string  `json:"reasoning"`
}

// NewLLMClient creates a new LLM client.
func NewLLMClient(apiKey string) (*LLMClient, error) {
	provider, resolvedKey, err := ResolveProvider(apiKey)
	if err != nil {
		return nil, err
	}

	client := &LLMClient{
		provider: provider,
		apiKey:   resolvedKey,
	}

	switch provider {
	case ProviderGemini:
		ctx := context.Background()
		geminiClient, err := genai.NewClient(ctx, option.WithAPIKey(resolvedKey))
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini client: %w", err)
		}
		client.gemini = geminiClient
		client.model = "gemini-2.0-flash-lite"
	case ProviderOpenAI:
		client.openAI = &http.Client{Timeout: 60 * time.Second}
		client.model = "gpt-5.2"
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	return client, nil
}

// Close closes underlying provider clients.
func (l *LLMClient) Close() error {
	if l.gemini != nil {
		return l.gemini.Close()
	}
	return nil
}

// Provider returns the resolved provider.
func (l *LLMClient) Provider() string {
	return string(l.provider)
}

// Model returns the resolved model.
func (l *LLMClient) Model() string {
	return l.model
}

// AnalyzeIssue performs triage analysis on an issue.
func (l *LLMClient) AnalyzeIssue(ctx context.Context, issue *IssueInput) (*TriageResult, error) {
	prompt := buildTriagePromptJSON(issue)

	responseText, err := l.generateText(ctx, prompt, 0.3, true)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze issue: %w", err)
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

	responseText, err := l.generateText(ctx, prompt, 0.5, false)
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %w", err)
	}

	return strings.TrimSpace(responseText), nil
}

// RouteIssue analyzes issue intent and ranks repositories by relevance.
func (l *LLMClient) RouteIssue(ctx context.Context, input *RouteIssueInput) (*RouterResult, error) {
	if len(input.Repositories) == 0 {
		return &RouterResult{Rankings: []RepositoryRanking{}, BestMatch: nil}, nil
	}

	prompt := buildRouteIssuePrompt(input)

	responseText, err := l.generateText(ctx, prompt, 0.3, true)
	if err != nil {
		return nil, fmt.Errorf("failed to route issue: %w", err)
	}

	var result struct {
		Rankings []RepositoryRanking `json:"rankings"`
	}
	if err := unmarshalJSONResponse(responseText, &result); err != nil {
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

	responseText, err := l.generateText(ctx, prompt, 0.3, true)
	if err != nil {
		return nil, fmt.Errorf("failed to assess quality: %w", err)
	}

	var result QualityResult
	if err := unmarshalJSONResponse(responseText, &result); err != nil {
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

	responseText, err := l.generateText(ctx, prompt, 0.3, true)
	if err != nil {
		return nil, fmt.Errorf("failed to detect duplicate: %w", err)
	}

	var result DuplicateResult
	if err := unmarshalJSONResponse(responseText, &result); err != nil {
		return nil, fmt.Errorf("failed to parse duplicate response: %w", err)
	}

	// Ensure non-nil SimilarIssues
	if result.SimilarIssues == nil {
		result.SimilarIssues = json.RawMessage("[]")
	}

	return &result, nil
}

// DetectPRDuplicate analyzes whether a pull request is a duplicate of existing issues/PRs.
func (l *LLMClient) DetectPRDuplicate(ctx context.Context, input *PRDuplicateCheckInput) (*PRDuplicateResult, error) {
	if len(input.Candidates) == 0 {
		return &PRDuplicateResult{IsDuplicate: false}, nil
	}

	prompt := buildPRDuplicateDetectionPrompt(input)

	responseText, err := l.generateText(ctx, prompt, 0.2, true)
	if err != nil {
		return nil, fmt.Errorf("failed to detect PR duplicate: %w", err)
	}

	var result PRDuplicateResult
	if err := unmarshalJSONResponse(responseText, &result); err != nil {
		return nil, fmt.Errorf("failed to parse PR duplicate response: %w", err)
	}

	if !result.IsDuplicate {
		result.DuplicateID = ""
	}

	return &result, nil
}

func (l *LLMClient) generateText(ctx context.Context, prompt string, temperature float32, jsonMode bool) (string, error) {
	switch l.provider {
	case ProviderGemini:
		return l.generateGeminiText(ctx, prompt, temperature, jsonMode)
	case ProviderOpenAI:
		return l.generateOpenAIText(ctx, prompt, temperature, jsonMode)
	default:
		return "", fmt.Errorf("unsupported provider: %s", l.provider)
	}
}

func (l *LLMClient) generateGeminiText(ctx context.Context, prompt string, temperature float32, jsonMode bool) (string, error) {
	model := l.gemini.GenerativeModel(l.model)
	model.SetTemperature(temperature)
	if jsonMode {
		model.ResponseMIMEType = "application/json"
	}

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	var responseText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			responseText += string(txt)
		}
	}

	return responseText, nil
}

func (l *LLMClient) generateOpenAIText(ctx context.Context, prompt string, temperature float32, jsonMode bool) (string, error) {
	type openAIMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type openAIResponseFormat struct {
		Type string `json:"type"`
	}
	type openAIRequest struct {
		Model          string                `json:"model"`
		Messages       []openAIMessage       `json:"messages"`
		Temperature    *float32              `json:"temperature,omitempty"`
		ResponseFormat *openAIResponseFormat `json:"response_format,omitempty"`
	}
	type openAIResponse struct {
		Choices []struct {
			Message struct {
				Content interface{} `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	temp := temperature
	req := openAIRequest{
		Model:       l.model,
		Messages:    []openAIMessage{{Role: "user", Content: prompt}},
		Temperature: &temp,
	}
	if jsonMode {
		req.ResponseFormat = &openAIResponseFormat{Type: "json_object"}
	}

	var resp openAIResponse
	if err := callOpenAIJSON(ctx, l.openAI, l.apiKey, "/v1/chat/completions", req, &resp); err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	responseText := strings.TrimSpace(extractOpenAIContent(resp.Choices[0].Message.Content))
	if responseText == "" {
		return "", fmt.Errorf("empty response from LLM")
	}

	return responseText, nil
}

func extractOpenAIContent(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			obj, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := obj["text"].(string); ok && text != "" {
				parts = append(parts, text)
				continue
			}
			if nested, ok := obj["content"].(string); ok && nested != "" {
				parts = append(parts, nested)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]interface{}:
		if text, ok := v["text"].(string); ok {
			return text
		}
	}
	return ""
}

func unmarshalJSONResponse(response string, out interface{}) error {
	cleaned := strings.TrimSpace(response)
	if err := json.Unmarshal([]byte(cleaned), out); err == nil {
		return nil
	}

	trimmed := trimCodeFence(cleaned)
	if err := json.Unmarshal([]byte(trimmed), out); err == nil {
		return nil
	}

	objectText := extractJSONObject(trimmed)
	if objectText != "" {
		if err := json.Unmarshal([]byte(objectText), out); err == nil {
			return nil
		}
	}

	return json.Unmarshal([]byte(cleaned), out)
}

func trimCodeFence(text string) string {
	s := strings.TrimSpace(text)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func extractJSONObject(text string) string {
	first := strings.IndexAny(text, "{[")
	lastObj := strings.LastIndexAny(text, "}]")
	if first >= 0 && lastObj > first {
		return strings.TrimSpace(text[first : lastObj+1])
	}
	return ""
}

// parseTriageResponseJSON parses the JSON LLM response into a TriageResult.
func parseTriageResponseJSON(response string) (*TriageResult, error) {
	var result TriageResult
	if err := unmarshalJSONResponse(response, &result); err != nil {
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
