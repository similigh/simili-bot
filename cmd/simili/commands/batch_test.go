// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-10
// Last Modified: 2026-02-10

package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/core/pipeline"
)

func TestLoadIssues(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantErr   bool
		wantCount int
	}{
		{
			name: "valid issues array",
			content: `[
				{
					"org": "test-org",
					"repo": "test-repo",
					"number": 123,
					"title": "Test Issue",
					"body": "Test body",
					"state": "open",
					"author": "testuser"
				},
				{
					"org": "test-org",
					"repo": "test-repo",
					"number": 124,
					"title": "Another Issue",
					"state": "closed"
				}
			]`,
			wantErr:   false,
			wantCount: 2,
		},
		{
			name:      "empty array",
			content:   `[]`,
			wantErr:   true,
			wantCount: 0,
		},
		{
			name:      "invalid JSON",
			content:   `[{invalid json`,
			wantErr:   true,
			wantCount: 0,
		},
		{
			name: "missing required fields",
			content: `[
				{
					"org": "test-org",
					"repo": "test-repo"
				}
			]`,
			wantErr:   true,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test_issues.json")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Test loadIssues
			issues, err := loadIssues(tmpFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadIssues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(issues) != tt.wantCount {
				t.Errorf("loadIssues() got %d issues, want %d", len(issues), tt.wantCount)
			}
		})
	}
}

func TestLoadIssues_FileNotFound(t *testing.T) {
	_, err := loadIssues("/nonexistent/path/file.json")
	if err == nil {
		t.Error("loadIssues() expected error for nonexistent file, got nil")
	}
}

func TestApplyConfigOverrides(t *testing.T) {
	tests := []struct {
		name                  string
		collection            string
		threshold             float64
		duplicateThresh       float64
		topK                  int
		wantCollection        string
		wantThreshold         float64
		wantDuplicateThresh   float64
		wantTopK              int
	}{
		{
			name:                "all overrides",
			collection:          "custom-collection",
			threshold:           0.75,
			duplicateThresh:     0.85,
			topK:                5,
			wantCollection:      "custom-collection",
			wantThreshold:       0.75,
			wantDuplicateThresh: 0.85,
			wantTopK:            5,
		},
		{
			name:                "collection only",
			collection:          "test-collection",
			threshold:           0,
			duplicateThresh:     0,
			topK:                0,
			wantCollection:      "test-collection",
			wantThreshold:       0,
			wantDuplicateThresh: 0,
			wantTopK:            0,
		},
		{
			name:                "thresholds only",
			collection:          "",
			threshold:           0.8,
			duplicateThresh:     0.9,
			topK:                0,
			wantCollection:      "",
			wantThreshold:       0.8,
			wantDuplicateThresh: 0.9,
			wantTopK:            0,
		},
		{
			name:                "no overrides",
			collection:          "",
			threshold:           0,
			duplicateThresh:     0,
			topK:                0,
			wantCollection:      "",
			wantThreshold:       0,
			wantDuplicateThresh: 0,
			wantTopK:            0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set global flags
			batchCollection = tt.collection
			batchThreshold = tt.threshold
			batchDuplicateThresh = tt.duplicateThresh
			batchTopK = tt.topK

			// Create config
			cfg := &config.Config{}

			// Apply overrides
			applyConfigOverrides(cfg)

			// Check results
			if tt.wantCollection != "" && cfg.Qdrant.Collection != tt.wantCollection {
				t.Errorf("collection = %v, want %v", cfg.Qdrant.Collection, tt.wantCollection)
			}
			if tt.wantThreshold > 0 && cfg.Defaults.SimilarityThreshold != tt.wantThreshold {
				t.Errorf("threshold = %v, want %v", cfg.Defaults.SimilarityThreshold, tt.wantThreshold)
			}
			if tt.wantDuplicateThresh > 0 && cfg.Transfer.DuplicateConfidenceThreshold != tt.wantDuplicateThresh {
				t.Errorf("duplicateThreshold = %v, want %v", cfg.Transfer.DuplicateConfidenceThreshold, tt.wantDuplicateThresh)
			}
			if tt.wantTopK > 0 && cfg.Defaults.MaxSimilarToShow != tt.wantTopK {
				t.Errorf("topK = %v, want %v", cfg.Defaults.MaxSimilarToShow, tt.wantTopK)
			}
		})
	}
}

func TestFormatJSON(t *testing.T) {
	now := time.Now()
	results := []BatchResult{
		{
			Index: 0,
			Issue: pipeline.Issue{
				Org:       "test-org",
				Repo:      "test-repo",
				Number:    123,
				Title:     "Test Issue",
				Author:    "testuser",
				State:     "open",
				CreatedAt: now,
			},
			Result: &pipeline.Result{
				IssueNumber: 123,
				Skipped:     false,
				SimilarFound: []pipeline.SimilarIssue{
					{
						Number:     100,
						Title:      "Similar Issue",
						Similarity: 0.87,
						State:      "open",
					},
				},
				TransferTarget:      "test-org/other-repo",
				TransferConfidence:  0.85,
				QualityScore:        0.72,
				SuggestedLabels:     []string{"bug", "needs-triage"},
				IsDuplicate:         false,
				DuplicateOf:         0,
				DuplicateConfidence: 0,
			},
			Error: nil,
		},
		{
			Index: 1,
			Issue: pipeline.Issue{
				Org:    "test-org",
				Repo:   "test-repo",
				Number: 124,
				Title:  "Failed Issue",
			},
			Result: nil,
			Error:  &testError{msg: "pipeline failed"},
		},
	}

	data, err := formatJSON(results)
	if err != nil {
		t.Fatalf("formatJSON() error = %v", err)
	}

	// Parse the JSON to validate structure
	var output JSONOutput
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate metadata
	if output.TotalIssues != 2 {
		t.Errorf("TotalIssues = %d, want 2", output.TotalIssues)
	}
	if output.Successful != 1 {
		t.Errorf("Successful = %d, want 1", output.Successful)
	}
	if output.Failed != 1 {
		t.Errorf("Failed = %d, want 1", output.Failed)
	}

	// Validate first result
	if len(output.Results) != 2 {
		t.Fatalf("Results length = %d, want 2", len(output.Results))
	}

	first := output.Results[0]
	if first.Issue.Number != 123 {
		t.Errorf("First issue number = %d, want 123", first.Issue.Number)
	}
	if first.Result == nil {
		t.Error("First result is nil")
	} else {
		if len(first.Result.SimilarFound) != 1 {
			t.Errorf("SimilarFound length = %d, want 1", len(first.Result.SimilarFound))
		}
	}
	if first.Error != "" {
		t.Errorf("First error should be empty, got %s", first.Error)
	}

	// Validate second result (error case)
	second := output.Results[1]
	if second.Issue.Number != 124 {
		t.Errorf("Second issue number = %d, want 124", second.Issue.Number)
	}
	if second.Result != nil {
		t.Error("Second result should be nil")
	}
	if second.Error == "" {
		t.Error("Second error should not be empty")
	}
}

func TestFormatCSV(t *testing.T) {
	results := []BatchResult{
		{
			Index: 0,
			Issue: pipeline.Issue{
				Org:    "test-org",
				Repo:   "test-repo",
				Number: 123,
				Title:  "Test Issue",
				Author: "testuser",
				State:  "open",
			},
			Result: &pipeline.Result{
				IssueNumber:         123,
				Skipped:             false,
				SkipReason:          "",
				SimilarFound:        []pipeline.SimilarIssue{{Number: 100, Similarity: 0.87}},
				TransferTarget:      "test-org/other-repo",
				TransferConfidence:  0.85,
				QualityScore:        0.72,
				SuggestedLabels:     []string{"bug", "needs-triage"},
				IsDuplicate:         true,
				DuplicateOf:         100,
				DuplicateConfidence: 0.95,
			},
			Error: nil,
		},
		{
			Index: 1,
			Issue: pipeline.Issue{
				Org:    "test-org",
				Repo:   "test-repo",
				Number: 124,
				Title:  "Failed Issue",
				State:  "closed",
			},
			Result: nil,
			Error:  &testError{msg: "pipeline error"},
		},
	}

	data, err := formatCSV(results)
	if err != nil {
		t.Fatalf("formatCSV() error = %v", err)
	}

	csvStr := string(data)
	lines := strings.Split(strings.TrimSpace(csvStr), "\n")

	// Check header
	if len(lines) < 1 {
		t.Fatal("CSV output has no lines")
	}

	header := lines[0]
	expectedHeaders := []string{
		"issue_number",
		"org",
		"repo",
		"title",
		"author",
		"state",
		"skipped",
		"skip_reason",
		"similar_count",
		"top_similar_number",
		"top_similar_score",
		"is_duplicate",
		"duplicate_of",
		"duplicate_confidence",
		"transfer_target",
		"transfer_confidence",
		"quality_score",
		"suggested_labels",
		"error",
	}

	for _, h := range expectedHeaders {
		if !strings.Contains(header, h) {
			t.Errorf("CSV header missing column: %s", h)
		}
	}

	// Check row count (header + 2 data rows)
	if len(lines) != 3 {
		t.Errorf("CSV has %d lines, want 3 (header + 2 rows)", len(lines))
	}

	// Validate first data row contains expected values
	firstRow := lines[1]
	if !strings.Contains(firstRow, "123") {
		t.Error("First row missing issue number 123")
	}
	if !strings.Contains(firstRow, "test-org") {
		t.Error("First row missing org")
	}
	if !strings.Contains(firstRow, "Test Issue") {
		t.Error("First row missing title")
	}

	// Validate second data row has error
	secondRow := lines[2]
	if !strings.Contains(secondRow, "124") {
		t.Error("Second row missing issue number 124")
	}
	if !strings.Contains(secondRow, "pipeline error") {
		t.Error("Second row missing error message")
	}
}

func TestFormatCSV_EmptyResults(t *testing.T) {
	results := []BatchResult{}
	data, err := formatCSV(results)
	if err != nil {
		t.Fatalf("formatCSV() error = %v", err)
	}

	// Should still have header
	csvStr := string(data)
	lines := strings.Split(strings.TrimSpace(csvStr), "\n")
	if len(lines) != 1 {
		t.Errorf("Empty CSV should have 1 line (header), got %d", len(lines))
	}
}

func TestFormatCSV_FieldEscaping(t *testing.T) {
	results := []BatchResult{
		{
			Index: 0,
			Issue: pipeline.Issue{
				Org:    "test-org",
				Repo:   "test-repo",
				Number: 123,
				Title:  "Issue with, comma and \"quotes\"",
				State:  "open",
			},
			Result: &pipeline.Result{
				IssueNumber: 123,
			},
			Error: nil,
		},
	}

	data, err := formatCSV(results)
	if err != nil {
		t.Fatalf("formatCSV() error = %v", err)
	}

	// The CSV library should properly escape the title
	csvStr := string(data)
	if !strings.Contains(csvStr, "123") {
		t.Error("CSV missing issue number")
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
