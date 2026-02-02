// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps provides the response builder step.
package steps

import (
	"fmt"
	"log"
	"strings"

	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
)

// ResponseBuilder constructs the comment to post on the issue.
type ResponseBuilder struct {
	llm *gemini.LLMClient
}

// NewResponseBuilder creates a new response builder step.
func NewResponseBuilder(deps *pipeline.Dependencies) *ResponseBuilder {
	return &ResponseBuilder{
		llm: deps.LLMClient,
	}
}

// Name returns the step name.
func (s *ResponseBuilder) Name() string {
	return "response_builder"
}

// Run builds the response comment.
func (s *ResponseBuilder) Run(ctx *pipeline.Context) error {
	if len(ctx.SimilarIssues) == 0 && ctx.TransferTarget == "" {
		log.Printf("[response_builder] No similar issues or transfer target, skipping comment")
		return nil
	}

	// Try AI response if LLM is available and we have similar issues
	if s.llm != nil && len(ctx.SimilarIssues) > 0 {
		aiComment := s.generateAIResponse(ctx)
		if aiComment != "" {
			ctx.Metadata["comment"] = aiComment
			log.Printf("[response_builder] Built AI comment")
			return nil
		}
	}

	// Fallback to template-based response
	comment := s.buildTemplateResponse(ctx)
	ctx.Metadata["comment"] = comment
	log.Printf("[response_builder] Built template comment with %d similar issues", len(ctx.SimilarIssues))

	return nil
}

func (s *ResponseBuilder) generateAIResponse(ctx *pipeline.Context) string {
	// Convert similar issues to Gemini input format
	similarInput := make([]gemini.SimilarIssueInput, len(ctx.SimilarIssues))
	for i, sim := range ctx.SimilarIssues {
		similarInput[i] = gemini.SimilarIssueInput{
			Number:     sim.Number,
			Title:      sim.Title,
			URL:        sim.URL,
			Similarity: sim.Similarity,
			State:      sim.State,
		}
	}

	response, err := s.llm.GenerateResponse(ctx.Ctx, similarInput)
	if err != nil {
		log.Printf("[response_builder] Failed to generate AI response: %v", err)
		return ""
	}

	// Add Simili footer
	return fmt.Sprintf("%s\n\n*(AI-generated response by Simili)*", response)
}

func (s *ResponseBuilder) buildTemplateResponse(ctx *pipeline.Context) string {
	var parts []string

	// Similar issues section
	if len(ctx.SimilarIssues) > 0 {
		parts = append(parts, "## 🔍 Similar Issues Found\n")
		parts = append(parts, "I found some issues that might be related:\n")
		for _, similar := range ctx.SimilarIssues {
			parts = append(parts, fmt.Sprintf("- [#%d](%s) - %s (%.0f%% similar)",
				similar.Number, similar.URL, similar.Title, similar.Similarity*100))
		}
		parts = append(parts, "")
	}

	// Transfer notification
	if ctx.TransferTarget != "" {
		parts = append(parts, "## 📦 Transfer Suggested\n")
		parts = append(parts, fmt.Sprintf("This issue may belong in **%s**.\n", ctx.TransferTarget))
	}

	return strings.Join(parts, "\n")
}
