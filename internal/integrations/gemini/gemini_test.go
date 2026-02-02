// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package gemini

import (
	"strings"
	"testing"
)

func TestBuildTriagePrompt(t *testing.T) {
	issue := &IssueInput{
		Title:  "Bug: Application crashes on startup",
		Body:   "When I start the application, it immediately crashes with error XYZ",
		Author: "testuser",
		Labels: []string{"bug", "needs-triage"},
	}

	prompt := buildTriagePrompt(issue)

	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}

	// Check that prompt contains key information
	if !strings.Contains(prompt, issue.Title) {
		t.Error("Prompt should contain issue title")
	}
	if !strings.Contains(prompt, issue.Author) {
		t.Error("Prompt should contain author")
	}
}

func TestBuildResponsePrompt(t *testing.T) {
	similar := []SimilarIssueInput{
		{
			Number:     123,
			Title:      "Similar issue",
			URL:        "https://github.com/org/repo/issues/123",
			Similarity: 0.85,
			State:      "closed",
		},
		{
			Number:     456,
			Title:      "Another similar issue",
			URL:        "https://github.com/org/repo/issues/456",
			Similarity: 0.75,
			State:      "open",
		},
	}

	prompt := buildResponsePrompt(similar)

	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}

	// Check that prompt contains issue information
	if !strings.Contains(prompt, "#123") {
		t.Error("Prompt should contain first issue number")
	}
	if !strings.Contains(prompt, "#456") {
		t.Error("Prompt should contain second issue number")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long string",
			input:    "hello world this is a long string",
			maxLen:   10,
			expected: "hello worl...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParseTriageResponse(t *testing.T) {
	tests := []struct {
		name            string
		response        string
		expectedQuality string
		expectedLabels  int
		expectedDupe    bool
	}{
		{
			name:            "good quality bug",
			response:        "Quality: good\nLabels: bug\nReasoning: Well described issue with clear steps",
			expectedQuality: "good",
			expectedLabels:  1,
			expectedDupe:    false,
		},
		{
			name:            "poor quality",
			response:        "Quality: poor\nLabels: question\nReasoning: Vague description, no details",
			expectedQuality: "poor",
			expectedLabels:  1,
			expectedDupe:    false,
		},
		{
			name:            "duplicate detection",
			response:        "Quality: good\nThis appears to be a duplicate of issue #123",
			expectedQuality: "good",
			expectedLabels:  0,
			expectedDupe:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTriageResponseLegacy(tt.response)

			if result.Quality != tt.expectedQuality {
				t.Errorf("Expected quality %q, got %q", tt.expectedQuality, result.Quality)
			}

			if len(result.SuggestedLabels) != tt.expectedLabels {
				t.Errorf("Expected %d labels, got %d", tt.expectedLabels, len(result.SuggestedLabels))
			}

			if result.IsDuplicate != tt.expectedDupe {
				t.Errorf("Expected duplicate=%v, got %v", tt.expectedDupe, result.IsDuplicate)
			}
		})
	}
}

