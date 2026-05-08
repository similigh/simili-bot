// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-04
// Last Modified: 2026-03-06

package steps

import (
	"context"
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/ai"
)

// duplicateDetectorLLM is the subset of ai.LLMClient used by DuplicateDetector.
// Using an interface here enables unit-testing without a real LLM connection.
type duplicateDetectorLLM interface {
	DetectDuplicate(ctx context.Context, input *ai.DuplicateCheckInput) (*ai.DuplicateResult, error)
}

// DuplicateDetector analyzes similarity results for duplicates using LLM.
type DuplicateDetector struct {
	llm duplicateDetectorLLM
}

// NewDuplicateDetector creates a new duplicate detector step.
func NewDuplicateDetector(deps *pipeline.Dependencies) *DuplicateDetector {
	return &DuplicateDetector{
		llm: deps.LLMClient,
	}
}

// Name returns the step name.
func (s *DuplicateDetector) Name() string {
	return "duplicate_detector"
}

// Run analyzes similar issues for duplicates.
func (s *DuplicateDetector) Run(ctx *pipeline.Context) error {
	// Only run on new issues, skip for comments/commands
	if ctx.Issue.EventType == "issue_comment" || ctx.Issue.EventType == "pr_comment" {
		return nil
	}
	if s.llm == nil {
		log.Printf("[duplicate_detector] No LLM client, skipping duplicate detection")
		return nil
	}

	// Skip if transfer scheduled
	if ctx.TransferTarget != "" {
		log.Printf("[duplicate_detector] Transfer scheduled, skipping duplicate detection")
		return nil
	}

	// Skip if no similar issues
	if len(ctx.SimilarIssues) == 0 {
		log.Printf("[duplicate_detector] No similar issues, skipping")
		return nil
	}

	log.Printf("[duplicate_detector] Analyzing %d similar issues for duplicates", len(ctx.SimilarIssues))

	// Use configured candidate limit (default 5 per issue #56).
	maxSimilar := ctx.Config.Defaults.DuplicateCandidates
	if maxSimilar <= 0 {
		maxSimilar = 5
	}
	if len(ctx.SimilarIssues) < maxSimilar {
		maxSimilar = len(ctx.SimilarIssues)
	}

	similarInput := make([]ai.SimilarIssueInput, maxSimilar)
	for i := 0; i < maxSimilar; i++ {
		similarInput[i] = ai.SimilarIssueInput{
			Number:     ctx.SimilarIssues[i].Number,
			Title:      ctx.SimilarIssues[i].Title,
			Body:       ctx.SimilarIssues[i].Body,
			URL:        ctx.SimilarIssues[i].URL,
			Similarity: ctx.SimilarIssues[i].Similarity,
			State:      ctx.SimilarIssues[i].State,
		}
	}

	input := &ai.DuplicateCheckInput{
		CurrentIssue: &ai.IssueInput{
			Title:  ctx.Issue.Title,
			Body:   ctx.Issue.Body,
			Author: ctx.Issue.Author,
			Labels: ctx.Issue.Labels,
		},
		SimilarIssues: similarInput,
	}

	result, err := s.llm.DetectDuplicate(ctx.Ctx, input)
	if err != nil {
		log.Printf("[duplicate_detector] Failed to detect duplicates: %v (non-blocking)", err)
		return nil // Graceful degradation
	}

	// Store full result and related issues in context.
	ctx.Metadata["duplicate_result"] = result
	ctx.Metadata["related_issues"] = result.RelatedIssues

	// Get threshold from config (default 0.85 to align with prompt guidance).
	threshold := ctx.Config.Transfer.DuplicateConfidenceThreshold
	if threshold == 0 {
		threshold = 0.85
	}

	// Store reasoning regardless of duplicate status
	ctx.Result.DuplicateReason = result.Reasoning

	// Mark as duplicate if high confidence
	if result.IsDuplicate && result.Confidence >= threshold {
		ctx.Result.IsDuplicate = true
		ctx.Result.DuplicateOf = result.DuplicateOf
		ctx.Result.DuplicateConfidence = result.Confidence
		// Add "potential-duplicate" label for the auto-close workflow
		ctx.Result.SuggestedLabels = append(ctx.Result.SuggestedLabels, "potential-duplicate")
		log.Printf("[duplicate_detector] Duplicate detected: #%d (%.2f confidence)",
			result.DuplicateOf, result.Confidence)
	} else {
		log.Printf("[duplicate_detector] No high-confidence duplicate found (reason: %s)", result.Reasoning)
	}

	return nil
}
