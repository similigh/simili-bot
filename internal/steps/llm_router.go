// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-04
// Last Modified: 2026-02-04

package steps

import (
	"fmt"
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
)

// LLMRouter analyzes issue intent and routes to best repository using LLM.
type LLMRouter struct {
	llm *gemini.LLMClient
}

// NewLLMRouter creates a new LLM router step.
func NewLLMRouter(deps *pipeline.Dependencies) *LLMRouter {
	return &LLMRouter{
		llm: deps.LLMClient,
	}
}

// Name returns the step name.
func (s *LLMRouter) Name() string {
	return "llm_router"
}

// Run analyzes issue and routes to best repository.
func (s *LLMRouter) Run(ctx *pipeline.Context) error {
	// Skip if LLM routing is disabled or no LLM client
	if ctx.Config.Transfer.LLMRoutingEnabled == nil || !*ctx.Config.Transfer.LLMRoutingEnabled || s.llm == nil {
		log.Printf("[llm_router] LLM routing disabled or no client, skipping")
		return nil
	}

	// Skip if transfer already determined by rules
	if ctx.TransferTarget != "" {
		log.Printf("[llm_router] Transfer already determined by rules, skipping LLM routing")
		return nil
	}

	log.Printf("[llm_router] Analyzing issue #%d for routing", ctx.Issue.Number)

	// Collect repository candidates (exclude current repo, require description)
	var candidates []gemini.RepositoryCandidate
	currentRepo := fmt.Sprintf("%s/%s", ctx.Issue.Org, ctx.Issue.Repo)

	for _, repo := range ctx.Config.Repositories {
		repoName := fmt.Sprintf("%s/%s", repo.Org, repo.Repo)
		if !repo.Enabled || repoName == currentRepo || repo.Description == "" {
			continue
		}
		candidates = append(candidates, gemini.RepositoryCandidate{
			Org:         repo.Org,
			Repo:        repo.Repo,
			Description: repo.Description,
		})
	}

	if len(candidates) == 0 {
		log.Printf("[llm_router] No candidate repositories with descriptions, skipping")
		return nil
	}

	// Call LLM to route issue
	input := &gemini.RouteIssueInput{
		Issue: &gemini.IssueInput{
			Title:  ctx.Issue.Title,
			Body:   ctx.Issue.Body,
			Author: ctx.Issue.Author,
			Labels: ctx.Issue.Labels,
		},
		Repositories: candidates,
	}

	result, err := s.llm.RouteIssue(ctx.Ctx, input)
	if err != nil {
		log.Printf("[llm_router] Failed to route issue: %v (non-blocking)", err)
		return nil // Graceful degradation
	}

	// Store result in metadata
	ctx.Metadata["router_result"] = result

	// Apply confidence-based action
	if result.BestMatch != nil {
		confidence := result.BestMatch.Confidence
		targetRepo := fmt.Sprintf("%s/%s", result.BestMatch.Org, result.BestMatch.Repo)

		ctx.Result.TransferConfidence = confidence
		ctx.Result.TransferReason = result.BestMatch.Reasoning

		if confidence >= ctx.Config.Transfer.MediumConfidence {
			// Proactive transfer: auto-transfer if confidence is medium or higher
			ctx.TransferTarget = targetRepo
			ctx.Result.TransferTarget = targetRepo
			ctx.Metadata["original_repo"] = fmt.Sprintf("%s/%s", ctx.Issue.Org, ctx.Issue.Repo)
			log.Printf("[llm_router] Proactive transfer (%.2f) to %s", confidence, targetRepo)
		} else {
			// Low confidence: silent
			log.Printf("[llm_router] Low confidence (%.2f), no action", confidence)
		}
	}

	return nil
}
