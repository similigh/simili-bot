// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps provides the triage step.
package steps

import (
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
)

// Triage uses LLM to analyze and classify the issue.
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

// Run analyzes the issue using LLM.
func (s *Triage) Run(ctx *pipeline.Context) error {
	if s.llm == nil {
		log.Printf("[triage] WARNING: No LLM client configured, skipping triage")
		return nil
	}

	log.Printf("[triage] Analyzing issue #%d", ctx.Issue.Number)

	// Convert pipeline.Issue to gemini.IssueInput
	input := &gemini.IssueInput{
		Title:  ctx.Issue.Title,
		Body:   ctx.Issue.Body,
		Author: ctx.Issue.Author,
		Labels: ctx.Issue.Labels,
	}

	result, err := s.llm.AnalyzeIssue(ctx.Ctx, input)
	if err != nil {
		log.Printf("[triage] Failed to analyze issue: %v", err)
		// We don't fail the pipeline, just skip triage results
		return nil
	}

	ctx.Result.SuggestedLabels = result.SuggestedLabels
	// TODO: Handle quality and reasoning (maybe store in metadata or comments)
	log.Printf("[triage] Analysis complete. Suggested Labels: %v", result.SuggestedLabels)

	return nil
}
