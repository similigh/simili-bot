// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-27

// Package transfer provides the VDB-based semantic transfer router.
package transfer

import (
	"context"
	"fmt"
	"log"

	"github.com/similigh/simili-bot/internal/integrations/ai"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
)

// VDBMatchResult contains the result of VDB-based transfer routing.
type VDBMatchResult struct {
	Target        string   // "owner/repo" of the suggested target
	Confidence    float64  // 0.0-1.0 confidence based on repo distribution
	SimilarIssues []string // IDs of similar issues from target repo
	Reasoning     string   // Optional LLM explanation
}

// Embedder is the subset of ai.Embedder used by VDBRouter.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// VDBRouter performs semantic transfer routing using vector similarity search.
type VDBRouter struct {
	embedder    Embedder
	vectorStore qdrant.VectorStore
	collection  string
	maxResults  int
}

// NewVDBRouter creates a new VDB router.
func NewVDBRouter(embedder Embedder, store qdrant.VectorStore, collection string, maxResults int) *VDBRouter {
	if maxResults <= 0 {
		maxResults = 50
	}
	return &VDBRouter{
		embedder:    embedder,
		vectorStore: store,
		collection:  collection,
		maxResults:  maxResults,
	}
}

// SuggestTransfer embeds the issue and analyses VDB results to propose a target repo.
// currentRepo is "owner/repo" and is excluded from the candidate set.
// Returns nil if no confident match is found.
func (r *VDBRouter) SuggestTransfer(ctx context.Context, issue *IssueInput, currentRepo string, confidenceThreshold float64, minSamples int, maxCandidates int) (*VDBMatchResult, error) {
	text := issue.Title
	if issue.Body != "" {
		text += "\n\n" + issue.Body
	}

	vec, err := r.embedder.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("vdb_router: embed failed: %w", err)
	}

	results, err := r.vectorStore.Search(ctx, r.collection, vec, r.maxResults, 0)
	if err != nil {
		return nil, fmt.Errorf("vdb_router: search failed: %w", err)
	}

	if len(results) == 0 {
		log.Printf("[vdb_router] No similar issues found in VDB")
		return nil, nil
	}

	// Count issues per repo, excluding current repo
	repoCounts := make(map[string]int)
	repoIDs := make(map[string][]string)
	for _, res := range results {
		org, _ := res.Payload["org"].(string)
		repo, _ := res.Payload["repo"].(string)
		if org == "" || repo == "" {
			continue
		}
		repoKey := org + "/" + repo
		if repoKey == currentRepo {
			continue
		}
		repoCounts[repoKey]++
		repoIDs[repoKey] = append(repoIDs[repoKey], res.ID)
	}

	if len(repoCounts) == 0 {
		log.Printf("[vdb_router] All similar issues belong to current repo, no transfer candidate")
		return nil, nil
	}

	// Find repo with most matches
	bestRepo := ""
	bestCount := 0
	for repo, count := range repoCounts {
		if count > bestCount {
			bestCount = count
			bestRepo = repo
		}
	}

	// Calculate total non-current results
	total := 0
	for _, c := range repoCounts {
		total += c
	}

	confidence := float64(bestCount) / float64(total)

	log.Printf("[vdb_router] Best candidate: %s (count=%d, total=%d, confidence=%.2f, threshold=%.2f)",
		bestRepo, bestCount, total, confidence, confidenceThreshold)

	if confidence < confidenceThreshold {
		log.Printf("[vdb_router] Confidence below threshold, no transfer suggested")
		return nil, nil
	}

	if bestCount < minSamples {
		log.Printf("[vdb_router] Sample count %d < minSamples %d, no transfer suggested", bestCount, minSamples)
		return nil, nil
	}

	ids := repoIDs[bestRepo]
	if len(ids) > maxCandidates {
		ids = ids[:maxCandidates]
	}

	return &VDBMatchResult{
		Target:        bestRepo,
		Confidence:    confidence,
		SimilarIssues: ids,
	}, nil
}

// VDBRouterWithGeminiEmbedder is a convenience constructor accepting *ai.Embedder directly.
func NewVDBRouterFromGemini(embedder *ai.Embedder, store qdrant.VectorStore, collection string, maxResults int) *VDBRouter {
	return NewVDBRouter(embedder, store, collection, maxResults)
}
