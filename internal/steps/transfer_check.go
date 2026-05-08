// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-27

// Package steps provides the transfer check step.
package steps

import (
	"fmt"
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/ai"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
	"github.com/similigh/simili-bot/internal/transfer"
)

// TransferCheck evaluates if an issue should be transferred to another repository.
// It first applies rule-based matching; if no rule matches and VDB routing is enabled
// it falls back to semantic VDB search (hybrid strategy).
type TransferCheck struct {
	embedder    *ai.Embedder
	vectorStore qdrant.VectorStore
	llmClient   *ai.LLMClient
}

// NewTransferCheck creates a new transfer check step.
func NewTransferCheck(deps *pipeline.Dependencies) *TransferCheck {
	return &TransferCheck{
		embedder:    deps.Embedder,
		vectorStore: deps.VectorStore,
		llmClient:   deps.LLMClient,
	}
}

// Name returns the step name.
func (s *TransferCheck) Name() string {
	return "transfer_check"
}

// Run checks if the issue should be transferred using transfer rules and optionally VDB routing.
func (s *TransferCheck) Run(ctx *pipeline.Context) error {
	if ctx.Issue.EventType == "pull_request" || ctx.Issue.EventType == "pr_comment" {
		log.Printf("[transfer_check] Pull request event detected, skipping transfer rules")
		return nil
	}

	// Skip if transfer is not enabled
	if ctx.Config.Transfer.Enabled == nil || !*ctx.Config.Transfer.Enabled {
		log.Printf("[transfer_check] Transfer not enabled, skipping")
		return nil
	}

	// Skip if a previous step (e.g. llm_router) already determined the transfer target
	if ctx.TransferTarget != "" {
		log.Printf("[transfer_check] Transfer already determined (%s), skipping", ctx.TransferTarget)
		return nil
	}

	// Check if transfer is blocked (e.g. by undo history)
	if blocked, _ := ctx.Metadata["transfer_blocked"].(bool); blocked {
		log.Printf("[transfer_check] Transfer blocked by metadata flag")
		return nil
	}

	blockedTargets, _ := ctx.Metadata["blocked_targets"].([]string)

	strategy := ctx.Config.Transfer.Strategy
	vdbCfg := ctx.Config.Transfer.VDBRouting
	vdbEnabled := vdbCfg.Enabled != nil && *vdbCfg.Enabled

	// Determine effective strategy
	if strategy == "" {
		if vdbEnabled {
			strategy = "hybrid"
		} else {
			strategy = "rules-only"
		}
	}

	log.Printf("[transfer_check] Strategy=%s vdbEnabled=%v issue=#%d", strategy, vdbEnabled, ctx.Issue.Number)

	// --- Rule-based matching ---
	ruleMatched := false
	if strategy != "vdb-only" && len(ctx.Config.Transfer.Rules) > 0 {
		matcher := transfer.NewRuleMatcher(ctx.Config.Transfer.Rules)
		input := &transfer.IssueInput{
			Title:  ctx.Issue.Title,
			Body:   ctx.Issue.Body,
			Labels: ctx.Issue.Labels,
			Author: ctx.Issue.Author,
		}
		result := matcher.Match(input)
		if result.Matched {
			if isBlockedTarget(result.Target, blockedTargets) {
				log.Printf("[transfer_check] Skipping transfer to %s: loop prevention (blocked target)", result.Target)
			} else {
				log.Printf("[transfer_check] Rule match: rule=%s target=%s", result.Rule.Name, result.Target)
				setTransferTarget(ctx, result.Target, "rule", 1.0, result.Rule.Name, "")
				ruleMatched = true
			}
		}
	}

	if ruleMatched {
		return nil
	}

	// --- VDB fallback ---
	if strategy == "rules-only" || !vdbEnabled {
		log.Printf("[transfer_check] VDB routing disabled or rules-only strategy, skipping VDB")
		return nil
	}

	if s.embedder == nil || s.vectorStore == nil {
		log.Printf("[transfer_check] VDB deps not available, skipping VDB routing")
		return nil
	}

	currentRepo := fmt.Sprintf("%s/%s", ctx.Issue.Org, ctx.Issue.Repo)
	collection := ctx.Config.Qdrant.Collection

	router := transfer.NewVDBRouter(s.embedder, s.vectorStore, collection, 50)

	vdbResult, err := router.SuggestTransfer(
		ctx.Ctx,
		&transfer.IssueInput{Title: ctx.Issue.Title, Body: ctx.Issue.Body},
		currentRepo,
		vdbCfg.ConfidenceThreshold,
		vdbCfg.MinSamplesPerRepo,
		vdbCfg.MaxCandidates,
	)
	if err != nil {
		log.Printf("[transfer_check] VDB routing error: %v", err)
		return nil // Non-fatal
	}

	if vdbResult == nil {
		log.Printf("[transfer_check] VDB found no confident transfer candidate")
		return nil
	}

	if isBlockedTarget(vdbResult.Target, blockedTargets) {
		log.Printf("[transfer_check] VDB target %s is blocked (loop prevention)", vdbResult.Target)
		return nil
	}

	// Optionally generate LLM explanation
	reasoning := ""
	if vdbCfg.ExplainDecision && s.llmClient != nil {
		similar := buildSimilarForExplain(vdbResult.SimilarIssues)
		explanation, err := s.llmClient.ExplainTransfer(ctx.Ctx, &ai.ExplainTransferInput{
			IssueTitle:    ctx.Issue.Title,
			IssueBody:     ctx.Issue.Body,
			TargetRepo:    vdbResult.Target,
			SimilarIssues: similar,
		})
		if err != nil {
			log.Printf("[transfer_check] ExplainTransfer error (non-fatal): %v", err)
		} else {
			reasoning = explanation
		}
	}

	log.Printf("[transfer_check] VDB transfer suggestion: target=%s confidence=%.2f",
		vdbResult.Target, vdbResult.Confidence)

	setTransferTarget(ctx, vdbResult.Target, "vdb", vdbResult.Confidence, "", reasoning)
	return nil
}

// setTransferTarget writes the transfer decision into the pipeline context.
func setTransferTarget(ctx *pipeline.Context, target, method string, confidence float64, ruleName, reasoning string) {
	ctx.TransferTarget = target
	ctx.Result.TransferTarget = target
	ctx.Result.TransferConfidence = confidence
	if reasoning != "" {
		ctx.Result.TransferReason = reasoning
	}

	// Store the current repo as original_repo so that response_builder can
	// correctly display "Transferred from <source>" regardless of whether the
	// transfer was determined by llm_router or transfer_check.
	ctx.Metadata["original_repo"] = fmt.Sprintf("%s/%s", ctx.Issue.Org, ctx.Issue.Repo)

	ctx.Metadata["transfer_method"] = method
	ctx.Metadata["transfer_confidence"] = confidence
	if reasoning != "" {
		ctx.Metadata["transfer_reasoning"] = reasoning
	}
	if ruleName != "" {
		ctx.Metadata["transfer_rule"] = ruleName
	}
	ctx.Metadata["skip_duplicate_detection"] = true
}

// isBlockedTarget checks if a target repo is in the blocked list.
func isBlockedTarget(target string, blocked []string) bool {
	for _, b := range blocked {
		if b == target {
			return true
		}
	}
	return false
}

// buildSimilarForExplain converts VDB result IDs to SimilarIssueInput stubs.
func buildSimilarForExplain(ids []string) []ai.SimilarIssueInput {
	out := make([]ai.SimilarIssueInput, 0, len(ids))
	for i, id := range ids {
		out = append(out, ai.SimilarIssueInput{
			Number: i + 1,
			Title:  id, // Best we have without full payloads
		})
	}
	return out
}
