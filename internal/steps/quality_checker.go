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

// QualityChecker assesses issue quality using LLM.
type QualityChecker struct {
	llm *gemini.LLMClient
}

// NewQualityChecker creates a new quality checker step.
func NewQualityChecker(deps *pipeline.Dependencies) *QualityChecker {
	return &QualityChecker{
		llm: deps.LLMClient,
	}
}

// Name returns the step name.
func (s *QualityChecker) Name() string {
	return "quality_checker"
}

// Run assesses issue quality.
func (s *QualityChecker) Run(ctx *pipeline.Context) error {
	if s.llm == nil {
		log.Printf("[quality_checker] No LLM client, skipping quality assessment")
		return nil
	}

	log.Printf("[quality_checker] Assessing quality for issue #%d", ctx.Issue.Number)

	input := &gemini.IssueInput{
		Title:  ctx.Issue.Title,
		Body:   ctx.Issue.Body,
		Author: ctx.Issue.Author,
		Labels: ctx.Issue.Labels,
	}

	result, err := s.llm.AssessQuality(ctx.Ctx, input)
	if err != nil {
		log.Printf("[quality_checker] Failed to assess quality: %v (non-blocking)", err)
		return nil // Graceful degradation
	}

	// Store in context
	ctx.Result.QualityScore = result.Score
	ctx.Result.QualityIssues = result.Issues
	ctx.Metadata["quality_result"] = result

	log.Printf("[quality_checker] Quality: %.2f (%s), Issues: %v",
		result.Score, result.Assessment, result.Issues)

	return nil
}
