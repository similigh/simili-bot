// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-04
// Last Modified: 2026-02-04

package steps

import (
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
)

// DuplicateDetector analyzes similarity results for duplicates using LLM.
type DuplicateDetector struct {
	llm *gemini.LLMClient
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

	// Convert similar issues to LLM input format (top 3 only)
	maxSimilar := 3
	if len(ctx.SimilarIssues) < maxSimilar {
		maxSimilar = len(ctx.SimilarIssues)
	}

	similarInput := make([]gemini.SimilarIssueInput, maxSimilar)
	for i := 0; i < maxSimilar; i++ {
		similarInput[i] = gemini.SimilarIssueInput{
			Number:     ctx.SimilarIssues[i].Number,
			Title:      ctx.SimilarIssues[i].Title,
			URL:        ctx.SimilarIssues[i].URL,
			Similarity: ctx.SimilarIssues[i].Similarity,
			State:      ctx.SimilarIssues[i].State,
		}
	}

	input := &gemini.DuplicateCheckInput{
		CurrentIssue: &gemini.IssueInput{
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

	// Store in context
	ctx.Metadata["duplicate_result"] = result

	// Get threshold from config (default 0.8)
	threshold := ctx.Config.Transfer.DuplicateConfidenceThreshold
	if threshold == 0 {
		threshold = 0.8
	}

	// Mark as duplicate if high confidence
	if result.IsDuplicate && result.Confidence >= threshold {
		ctx.Result.IsDuplicate = true
		ctx.Result.DuplicateOf = result.DuplicateOf
		ctx.Result.DuplicateConfidence = result.Confidence
		log.Printf("[duplicate_detector] Duplicate detected: #%d (%.2f confidence)",
			result.DuplicateOf, result.Confidence)
	} else {
		log.Printf("[duplicate_detector] No high-confidence duplicate found")
	}

	return nil
}
