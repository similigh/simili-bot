// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-03-05
// Last Modified: 2026-03-05

package commands

import (
	"encoding/json"
	"testing"

	"github.com/similigh/simili-bot/internal/integrations/qdrant"
)

func TestParsePRDuplicateOutput(t *testing.T) {
	out := &PRDuplicateOutput{
		PR: &PRRef{
			Repo:   "owner/repo",
			Number: 42,
			Title:  "Fix auth bug",
		},
		Candidates: []PRCandidate{
			{Type: "issue", Number: 10, Title: "Auth broken", Score: 0.91, URL: "https://github.com/owner/repo/issues/10"},
		},
		DuplicateDetected: true,
		DuplicateOf:       10,
		Confidence:        0.92,
		Reasoning:         "Same root cause",
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var got PRDuplicateOutput
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if got.PR.Number != 42 {
		t.Errorf("Expected PR.Number=42, got %d", got.PR.Number)
	}
	if len(got.Candidates) != 1 {
		t.Errorf("Expected 1 candidate, got %d", len(got.Candidates))
	}
	if got.Candidates[0].Score != 0.91 {
		t.Errorf("Expected score 0.91, got %f", got.Candidates[0].Score)
	}
	if !got.DuplicateDetected {
		t.Error("Expected DuplicateDetected=true")
	}
	if got.DuplicateOf != 10 {
		t.Errorf("Expected DuplicateOf=10, got %d", got.DuplicateOf)
	}
}

func TestMergeSearchResults(t *testing.T) {
	makeHit := func(id string, score float32, itemType string, number int, title, url string) *qdrant.SearchResult {
		return &qdrant.SearchResult{
			ID:    id,
			Score: score,
			Payload: map[string]any{
				"type":         itemType,
				"issue_number": number,
				"title":        title,
				"url":          url,
			},
		}
	}

	issueHits := []*qdrant.SearchResult{
		makeHit("a", 0.91, "issue", 10, "Auth broken", "https://github.com/owner/repo/issues/10"),
		makeHit("b", 0.75, "issue", 20, "Login fails", "https://github.com/owner/repo/issues/20"),
	}
	prHits := []*qdrant.SearchResult{
		makeHit("c", 0.88, "pull_request", 5, "Fix login", "https://github.com/owner/repo/pull/5"),
		// Duplicate of issue hit — same (type, number) pair.
		makeHit("d", 0.70, "issue", 10, "Auth broken (dup)", "https://github.com/owner/repo/issues/10"),
	}

	candidates := mergeSearchResults(issueHits, prHits, 99)

	// Expect 3 unique: issue:10, pr:5, issue:20 (dup issue:10 from prHits is dropped).
	if len(candidates) != 3 {
		t.Fatalf("Expected 3 candidates, got %d", len(candidates))
	}

	// Must be sorted by score descending.
	for i := 1; i < len(candidates); i++ {
		if candidates[i-1].Score < candidates[i].Score {
			t.Errorf("Candidates not sorted: index %d score %f < index %d score %f", i-1, candidates[i-1].Score, i, candidates[i].Score)
		}
	}

	// Top result must be issue #10 (score 0.91).
	if candidates[0].Number != 10 || candidates[0].Type != "issue" {
		t.Errorf("Expected top candidate issue #10, got %s #%d", candidates[0].Type, candidates[0].Number)
	}
}

func TestMergeSearchResultsExcludesCurrentPR(t *testing.T) {
	prHits := []*qdrant.SearchResult{
		{
			ID:    "self",
			Score: 0.99,
			Payload: map[string]any{
				"type":      "pull_request",
				"pr_number": 123,
				"title":     "Self reference",
				"url":       "https://github.com/owner/repo/pull/123",
			},
		},
	}

	candidates := mergeSearchResults(nil, prHits, 123)
	if len(candidates) != 0 {
		t.Errorf("Expected current PR to be excluded, got %d candidates", len(candidates))
	}
}

func TestMergeSearchResultsNilInputs(t *testing.T) {
	// Both nil — should not panic and return no candidates.
	candidates := mergeSearchResults(nil, nil, 1)
	if len(candidates) != 0 {
		t.Errorf("Expected 0 candidates for nil inputs, got %d", len(candidates))
	}
}

func TestMergeSearchResultsNumberKeyVariants(t *testing.T) {
	// Indexer step stores the key as "number"; similarity.go checks both
	// "number" and "issue_number". Verify payloadInt handles all variants.
	hits := []*qdrant.SearchResult{
		{
			ID:    "a",
			Score: 0.85,
			Payload: map[string]any{
				"type":   "issue",
				"number": 7, // primary key used by internal/steps/indexer.go
				"title":  "Via number key",
				"url":    "https://github.com/owner/repo/issues/7",
			},
		},
		{
			// Hit with no extractable number should be silently skipped.
			ID:    "b",
			Score: 0.80,
			Payload: map[string]any{
				"type":  "issue",
				"title": "No number",
				"url":   "https://github.com/owner/repo/issues/0",
			},
		},
	}

	candidates := mergeSearchResults(hits, nil, 99)
	if len(candidates) != 1 {
		t.Fatalf("Expected 1 candidate (zero-number hit skipped), got %d", len(candidates))
	}
	if candidates[0].Number != 7 {
		t.Errorf("Expected number=7, got %d", candidates[0].Number)
	}
}
