// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-04
// Last Modified: 2026-02-05

package steps

import (
	"fmt"
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
)

// LLMRouter analyzes issue intent and routes to best repository using LLM.
type LLMRouter struct {
	llm      *gemini.LLMClient
	embedder *gemini.Embedder
	store    qdrant.VectorStore
}

// NewLLMRouter creates a new LLM router step.
func NewLLMRouter(deps *pipeline.Dependencies) *LLMRouter {
	return &LLMRouter{
		llm:      deps.LLMClient,
		embedder: deps.Embedder,
		store:    deps.VectorStore,
	}
}

// Name returns the step name.
func (s *LLMRouter) Name() string {
	return "llm_router"
}

// Run analyzes issue and routes to best repository.
func (s *LLMRouter) Run(ctx *pipeline.Context) error {
	// Only run on new issues, skip for comments/commands and pull requests
	if ctx.Issue.EventType == "issue_comment" || ctx.Issue.EventType == "pull_request" || ctx.Issue.EventType == "pr_comment" {
		return nil
	}
	// Skip if LLM routing is disabled or no LLM client
	if ctx.Config.Transfer.LLMRoutingEnabled == nil || !*ctx.Config.Transfer.LLMRoutingEnabled || s.llm == nil {
		log.Printf("[llm_router] LLM routing disabled or no client, skipping")
		return nil
	}

	// Check if transfer is blocked (e.g. by undo history)
	if blocked, _ := ctx.Metadata["transfer_blocked"].(bool); blocked {
		log.Printf("[llm_router] Transfer blocked by metadata flag")
		return nil
	}

	// Skip if transfer already determined by rules
	if ctx.TransferTarget != "" {
		log.Printf("[llm_router] Transfer already determined by rules, skipping LLM routing")
		return nil
	}

	blockedTargets, _ := ctx.Metadata["blocked_targets"].([]string)

	log.Printf("[llm_router] Analyzing issue #%d for routing", ctx.Issue.Number)

	// Collect repository candidates (include current repo to allow "stay here" decision)
	var candidates []gemini.RepositoryCandidate
	currentRepo := fmt.Sprintf("%s/%s", ctx.Issue.Org, ctx.Issue.Repo)

	for _, repo := range ctx.Config.Repositories {
		if !repo.Enabled || repo.Description == "" {
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

	// Fetch repository definitions from repo collection (if available)
	repoDefinitions := make(map[string]string) // map[org/repo]full_definition

	if s.embedder != nil && s.store != nil && ctx.Config.Transfer.RepoCollection != "" {
		// Check if repo collection exists
		exists, err := s.store.CollectionExists(ctx.Ctx, ctx.Config.Transfer.RepoCollection)
		if err != nil {
			log.Printf("[llm_router] Error checking repo collection: %v (non-blocking)", err)
		} else if !exists {
			log.Printf("[llm_router] Repo collection '%s' not found, using config descriptions",
				ctx.Config.Transfer.RepoCollection)
		} else {
			// Generate embedding for issue to find relevant repos
			issueContent := fmt.Sprintf("%s\n\n%s", ctx.Issue.Title, ctx.Issue.Body)
			issueEmbedding, err := s.embedder.Embed(ctx.Ctx, issueContent)

			if err != nil {
				log.Printf("[llm_router] Error generating issue embedding: %v (non-blocking)", err)
			} else {
				// Search for relevant repository definitions
				// Use broader threshold (0.5) and higher limit (10) to get context
				results, err := s.store.Search(
					ctx.Ctx,
					ctx.Config.Transfer.RepoCollection,
					issueEmbedding,
					10,  // Get top 10 relevant repo docs (more than we'll route to)
					0.5, // Lower threshold than issues (0.65-0.7) for broader context
				)

				if err != nil {
					log.Printf("[llm_router] Error searching repo collection: %v (non-blocking)", err)
				} else if len(results) == 0 {
					log.Printf("[llm_router] No repository definitions found")
				} else {
					// Collect definitions per repo (handle multiple files per repo)
					// Max total size per repo to prevent token explosion
					const maxTotalSizePerRepo = 5000

					for _, res := range results {
						org, _ := res.Payload["org"].(string)
						repo, _ := res.Payload["repo"].(string)
						text, _ := res.Payload["text"].(string)
						file, _ := res.Payload["file"].(string)

						if org == "" || repo == "" || text == "" {
							log.Printf("[llm_router] Invalid repo definition payload, skipping")
							continue
						}

						repoKey := fmt.Sprintf("%s/%s", org, repo)

						// Build new definition with file header
						newDoc := fmt.Sprintf("--- %s ---\n\n%s", file, text)

						// If multiple docs for same repo, concatenate with size limit
						if existing, ok := repoDefinitions[repoKey]; ok {
							combined := existing + "\n\n" + newDoc
							// Enforce max total size per repo
							if len(combined) > maxTotalSizePerRepo {
								log.Printf("[llm_router] Truncating %s definition (size: %d > %d)",
									repoKey, len(combined), maxTotalSizePerRepo)
								combined = combined[:maxTotalSizePerRepo] + "\n... (truncated)"
							}
							repoDefinitions[repoKey] = combined
						} else {
							// First document for this repo
							if len(newDoc) > maxTotalSizePerRepo {
								newDoc = newDoc[:maxTotalSizePerRepo] + "\n... (truncated)"
							}
							repoDefinitions[repoKey] = newDoc
						}
					}
					log.Printf("[llm_router] Loaded %d repository definitions from %d documents",
						len(repoDefinitions), len(results))
				}
			}
		}
	}

	// Enhance candidates with full definitions
	for i := range candidates {
		repoKey := fmt.Sprintf("%s/%s", candidates[i].Org, candidates[i].Repo)
		if def, ok := repoDefinitions[repoKey]; ok {
			candidates[i].Definition = def
		}
	}

	// Call LLM to route issue (reuse currentRepo from above)
	input := &gemini.RouteIssueInput{
		Issue: &gemini.IssueInput{
			Title:  ctx.Issue.Title,
			Body:   ctx.Issue.Body,
			Author: ctx.Issue.Author,
			Labels: ctx.Issue.Labels,
		},
		Repositories: candidates,
		CurrentRepo:  currentRepo,
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

		// Only transfer if best match is a DIFFERENT repository
		if targetRepo == currentRepo {
			log.Printf("[llm_router] Issue belongs in current repo %s (%.2f confidence), no transfer needed",
				currentRepo, confidence)
			return nil
		}

		if confidence >= ctx.Config.Transfer.MediumConfidence {
			// Check if target is blocked
			for _, blocked := range blockedTargets {
				if blocked == targetRepo {
					log.Printf("[llm_router] Skipping proactive transfer to %s: detected loop (blocked target)", targetRepo)
					return nil
				}
			}

			// Proactive transfer: auto-transfer if confidence is medium or higher
			ctx.TransferTarget = targetRepo
			ctx.Result.TransferTarget = targetRepo
			ctx.Metadata["original_repo"] = currentRepo
			log.Printf("[llm_router] Proactive transfer (%.2f) from %s to %s", confidence, currentRepo, targetRepo)
		} else {
			// Low confidence: silent
			log.Printf("[llm_router] Low confidence (%.2f) for transfer to %s, keeping in %s", confidence, targetRepo, currentRepo)
		}
	}

	return nil
}
