// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-04

// Package steps provides the triage step.
package steps

import (
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
)

// Triage uses LLM to suggest labels for the issue.
type Triage struct {
	llm *gemini.LLMClient
}

// NewTriage creates a new triage step.
func NewTriage(deps *pipeline.Dependencies) *Triage {
	return &Triage{
		llm: deps.LLMClient,
	}
}

// Name returns the step name.
func (s *Triage) Name() string {
	return "triage"
}

// Run analyzes the issue for label suggestions.
// Quality assessment is now handled by the quality_checker step.
func (s *Triage) Run(ctx *pipeline.Context) error {
	if s.llm == nil {
		log.Printf("[triage] WARNING: No LLM client configured, skipping triage")
		return nil
	}

	log.Printf("[triage] Analyzing issue #%d for labels", ctx.Issue.Number)

	// Convert pipeline.Issue to gemini.IssueInput
	input := &gemini.IssueInput{
		Title:  ctx.Issue.Title,
		Body:   ctx.Issue.Body,
		Author: ctx.Issue.Author,
		Labels: ctx.Issue.Labels,
	}

	result, err := s.llm.AnalyzeIssue(ctx.Ctx, input)
	if err != nil {
		log.Printf("[triage] Failed to analyze issue: %v (non-blocking)", err)
		return nil // Graceful degradation
	}

	ctx.Result.SuggestedLabels = result.SuggestedLabels
	log.Printf("[triage] Suggested labels: %v", result.SuggestedLabels)

	return nil
}
