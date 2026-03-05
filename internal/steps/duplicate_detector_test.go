// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-03-06
// Last Modified: 2026-03-06

package steps

import (
	"context"
	"testing"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/ai"
)

// fakeLLM implements duplicateDetectorLLM for unit tests.
type fakeLLM struct {
	result        *ai.DuplicateResult
	err           error
	capturedInput *ai.DuplicateCheckInput
}

func (f *fakeLLM) DetectDuplicate(_ context.Context, input *ai.DuplicateCheckInput) (*ai.DuplicateResult, error) {
	f.capturedInput = input
	return f.result, f.err
}

// newTestCtx builds a minimal pipeline.Context with n similar issues.
func newTestCtx(cfg *config.Config, n int) *pipeline.Context {
	issue := &pipeline.Issue{
		Title:     "Test Issue",
		Body:      "Some body text",
		EventType: "issues",
	}
	ctx := pipeline.NewContext(context.Background(), issue, cfg)
	for i := 0; i < n; i++ {
		ctx.SimilarIssues = append(ctx.SimilarIssues, pipeline.SimilarIssue{
			Number:     i + 1,
			Title:      "Similar issue",
			Similarity: 0.9,
		})
	}
	return ctx
}

// newMinimalConfig returns a Config with only the fields used by DuplicateDetector.
func newMinimalConfig(candidates int, threshold float64) *config.Config {
	return &config.Config{
		Defaults: config.DefaultsConfig{
			DuplicateCandidates: candidates,
		},
		Transfer: config.TransferConfig{
			DuplicateConfidenceThreshold: threshold,
		},
	}
}

func TestDuplicateDetector_UsesConfigDuplicateCandidates(t *testing.T) {
	fake := &fakeLLM{result: &ai.DuplicateResult{RelatedIssues: []ai.RelatedIssueRef{}}}
	step := &DuplicateDetector{llm: fake}
	cfg := newMinimalConfig(2, 0.85)
	ctx := newTestCtx(cfg, 4)

	if err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.capturedInput == nil {
		t.Fatal("LLM was not called")
	}
	if got := len(fake.capturedInput.SimilarIssues); got != 2 {
		t.Errorf("expected 2 candidates sent to LLM, got %d", got)
	}
}

func TestDuplicateDetector_DefaultsToFiveWhenZero(t *testing.T) {
	fake := &fakeLLM{result: &ai.DuplicateResult{RelatedIssues: []ai.RelatedIssueRef{}}}
	step := &DuplicateDetector{llm: fake}
	// DuplicateCandidates=0 should fall back to 5
	cfg := newMinimalConfig(0, 0.85)
	ctx := newTestCtx(cfg, 6)

	if err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := len(fake.capturedInput.SimilarIssues); got != 5 {
		t.Errorf("expected 5 candidates sent to LLM, got %d", got)
	}
}

func TestDuplicateDetector_ConfidenceGateBlocks(t *testing.T) {
	fake := &fakeLLM{result: &ai.DuplicateResult{
		IsDuplicate:   true,
		DuplicateOf:   42,
		Confidence:    0.75, // below threshold of 0.8
		RelatedIssues: []ai.RelatedIssueRef{},
	}}
	step := &DuplicateDetector{llm: fake}
	cfg := newMinimalConfig(5, 0.8)
	ctx := newTestCtx(cfg, 1)

	if err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.Result.IsDuplicate {
		t.Error("expected IsDuplicate=false when confidence below threshold")
	}
}

func TestDuplicateDetector_RelatedIssuesStoredInMetadata(t *testing.T) {
	related := []ai.RelatedIssueRef{
		{Number: 10, Title: "Auth middleware", Relationship: "related"},
	}
	fake := &fakeLLM{result: &ai.DuplicateResult{
		IsDuplicate:   false,
		RelatedIssues: related,
	}}
	step := &DuplicateDetector{llm: fake}
	cfg := newMinimalConfig(5, 0.85)
	ctx := newTestCtx(cfg, 2)

	if err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored, ok := ctx.Metadata["related_issues"].([]ai.RelatedIssueRef)
	if !ok {
		t.Fatal("related_issues not stored in metadata")
	}
	if len(stored) != 1 || stored[0].Number != 10 {
		t.Errorf("unexpected related_issues in metadata: %+v", stored)
	}
}

func TestDuplicateDetector_RelatedNotMarkedDuplicate(t *testing.T) {
	// LLM returns IsDuplicate=true at 0.82 confidence (below 0.85 threshold).
	// The issue should NOT be marked duplicate but related issues should still persist.
	related := []ai.RelatedIssueRef{
		{Number: 7, Title: "Token refresh", Relationship: "related"},
	}
	fake := &fakeLLM{result: &ai.DuplicateResult{
		IsDuplicate:   true,
		DuplicateOf:   7,
		Confidence:    0.82,
		RelatedIssues: related,
	}}
	step := &DuplicateDetector{llm: fake}
	cfg := newMinimalConfig(5, 0.85)
	ctx := newTestCtx(cfg, 2)

	if err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.Result.IsDuplicate {
		t.Error("related issue at 0.82 confidence should not be marked as duplicate (threshold 0.85)")
	}

	stored, ok := ctx.Metadata["related_issues"].([]ai.RelatedIssueRef)
	if !ok || len(stored) == 0 {
		t.Error("related_issues should still be stored in metadata even when confidence gate blocks duplicate")
	}
}
