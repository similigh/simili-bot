// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-27

package transfer

import (
	"context"
	"testing"

	"github.com/similigh/simili-bot/internal/integrations/qdrant"
)

// --- mock embedder ---

type mockEmbedder struct {
	vec []float32
	err error
}

func (m *mockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return m.vec, m.err
}

// --- mock vector store ---

type mockVectorStore struct {
	results []*qdrant.SearchResult
	err     error
}

func (m *mockVectorStore) CreateCollection(_ context.Context, _ string, _ int) error { return nil }
func (m *mockVectorStore) CollectionExists(_ context.Context, _ string) (bool, error) {
	return true, nil
}
func (m *mockVectorStore) Upsert(_ context.Context, _ string, _ []*qdrant.Point) error { return nil }
func (m *mockVectorStore) Search(_ context.Context, _ string, _ []float32, _ int, _ float64) ([]*qdrant.SearchResult, error) {
	return m.results, m.err
}
func (m *mockVectorStore) Delete(_ context.Context, _ string, _ string) error { return nil }
func (m *mockVectorStore) SetPayload(_ context.Context, _ string, _ string, _ map[string]interface{}) error {
	return nil
}
func (m *mockVectorStore) Close() error { return nil }

// --- helpers ---

func makeResult(org, repo, id string, score float32) *qdrant.SearchResult {
	return &qdrant.SearchResult{
		ID:    id,
		Score: score,
		Payload: map[string]interface{}{
			"org":  org,
			"repo": repo,
		},
	}
}

func newTestRouter(results []*qdrant.SearchResult) *VDBRouter {
	return NewVDBRouter(
		&mockEmbedder{vec: []float32{0.1, 0.2, 0.3}},
		&mockVectorStore{results: results},
		"test_collection",
		50,
	)
}

func issueInput(title, body string) *IssueInput {
	return &IssueInput{Title: title, Body: body}
}

// TestNoResults: empty VDB returns nil.
func TestVDBRouter_NoResults(t *testing.T) {
	router := newTestRouter(nil)
	result, err := router.SuggestTransfer(context.Background(), issueInput("foo", "bar"), "org/current", 0.5, 1, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// TestAllSameRepo: all results in current repo → nil.
func TestVDBRouter_AllCurrentRepo(t *testing.T) {
	results := []*qdrant.SearchResult{
		makeResult("org", "current", "1", 0.9),
		makeResult("org", "current", "2", 0.85),
	}
	router := newTestRouter(results)
	result, err := router.SuggestTransfer(context.Background(), issueInput("foo", "bar"), "org/current", 0.5, 1, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for all-same-repo, got %+v", result)
	}
}

// TestClearWinner: one repo dominates → confident suggestion.
func TestVDBRouter_ClearWinner(t *testing.T) {
	results := []*qdrant.SearchResult{
		makeResult("org", "target", "1", 0.95),
		makeResult("org", "target", "2", 0.90),
		makeResult("org", "target", "3", 0.88),
		makeResult("org", "other", "4", 0.70),
	}
	router := newTestRouter(results)
	// 3/4 = 0.75 confidence; threshold 0.7, minSamples=1
	result, err := router.SuggestTransfer(context.Background(), issueInput("foo", "bar"), "org/current", 0.7, 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected a suggestion, got nil")
	}
	if result.Target != "org/target" {
		t.Errorf("expected org/target, got %s", result.Target)
	}
	if result.Confidence < 0.7 {
		t.Errorf("expected confidence >= 0.7, got %.2f", result.Confidence)
	}
}

// TestBelowConfidenceThreshold: winner doesn't meet threshold.
func TestVDBRouter_BelowThreshold(t *testing.T) {
	results := []*qdrant.SearchResult{
		makeResult("org", "repoA", "1", 0.9),
		makeResult("org", "repoB", "2", 0.85),
		makeResult("org", "repoC", "3", 0.80),
	}
	router := newTestRouter(results)
	// 1/3 ≈ 0.33 confidence; threshold 0.7
	result, err := router.SuggestTransfer(context.Background(), issueInput("foo", "bar"), "org/current", 0.7, 1, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil (below threshold), got %+v", result)
	}
}

// TestMinSamplesNotMet: not enough samples even if confident.
func TestVDBRouter_MinSamplesNotMet(t *testing.T) {
	results := []*qdrant.SearchResult{
		makeResult("org", "target", "1", 0.95),
	}
	router := newTestRouter(results)
	// 1/1 = 1.0 confidence but minSamples=5
	result, err := router.SuggestTransfer(context.Background(), issueInput("foo", "bar"), "org/current", 0.5, 5, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil (min samples not met), got %+v", result)
	}
}

// TestMaxCandidatesTruncation: returned IDs are capped at maxCandidates.
func TestVDBRouter_MaxCandidatesTruncation(t *testing.T) {
	results := []*qdrant.SearchResult{
		makeResult("org", "target", "1", 0.99),
		makeResult("org", "target", "2", 0.98),
		makeResult("org", "target", "3", 0.97),
		makeResult("org", "target", "4", 0.96),
		makeResult("org", "target", "5", 0.95),
	}
	router := newTestRouter(results)
	result, err := router.SuggestTransfer(context.Background(), issueInput("foo", "bar"), "org/current", 0.5, 1, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected suggestion")
	}
	if len(result.SimilarIssues) > 3 {
		t.Errorf("expected at most 3 similar issues, got %d", len(result.SimilarIssues))
	}
}
