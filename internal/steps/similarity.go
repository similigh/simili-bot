// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps provides the similarity search step.
package steps

import (
	"fmt"
	"log"
	"strings"

	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
)

// SimilaritySearch finds similar issues using the vector database.
type SimilaritySearch struct {
	embedder *gemini.Embedder
	store    qdrant.VectorStore
}

// NewSimilaritySearch creates a new similarity search step.
func NewSimilaritySearch(deps *pipeline.Dependencies) *SimilaritySearch {
	return &SimilaritySearch{
		embedder: deps.Embedder,
		store:    deps.VectorStore,
	}
}

// Name returns the step name.
func (s *SimilaritySearch) Name() string {
	return "similarity_search"
}

func normalizeSimilarThreadType(rawType string) string {
	switch strings.ToLower(strings.TrimSpace(rawType)) {
	case "pr", "pull_request", "pull request":
		return "pr"
	default:
		return "issue"
	}
}

// Run searches for similar issues.
func (s *SimilaritySearch) Run(ctx *pipeline.Context) error {
	// Skip if transfer is detected (duplicate detection not needed)
	if skip, ok := ctx.Metadata["skip_duplicate_detection"].(bool); ok && skip {
		log.Printf("[similarity_search] Skipping (transfer detected)")
		return nil
	}

	collectionName := ctx.Config.Qdrant.Collection
	threshold := ctx.Config.Defaults.SimilarityThreshold
	limit := ctx.Config.Defaults.MaxSimilarToShow

	// Cross-repo search logic could be added here if needed, but current implementation
	// depends on the collection containing all org issues.

	// Skip if dependencies are missing (e.g. testing mode)
	if s.embedder == nil || s.store == nil {
		log.Printf("[similarity_search] WARNING: Dependencies missing, skipping search")
		return nil
	}

	// Create content for embedding
	content := fmt.Sprintf("%s\n\n%s", ctx.Issue.Title, ctx.Issue.Body)

	// Generate embedding
	embedding, err := s.embedder.Embed(ctx.Ctx, content)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Search in Qdrant
	results, err := s.store.Search(ctx.Ctx, collectionName, embedding, limit, threshold)
	if err != nil {
		// Log error but don't fail pipeline? Or fail?
		// Failing is probably safer so we know somethings wrong.
		return fmt.Errorf("failed to search for similar issues: %w", err)
	}

	foundIssues := make([]pipeline.SimilarIssue, 0, len(results))
	for _, res := range results {
		// Safely extract payload fields with type checking
		// Match the indexer which uses "issue_number", and handle multiple numeric types
		var number int
		numFound := false

		for _, key := range []string{"number", "issue_number"} {
			if val, ok := res.Payload[key]; ok {
				switch v := val.(type) {
				case float64:
					number = int(v)
					numFound = true
				case int64:
					number = int(v)
					numFound = true
				case int:
					number = v
					numFound = true
				}
			}
			if numFound {
				break
			}
		}

		if !numFound {
			log.Printf("[similarity_search] WARNING: No valid issue number in payload, skipping result")
			continue
		}

		// Filter out the current issue itself
		resRepo, _ := res.Payload["repo"].(string)
		if number == ctx.Issue.Number && resRepo == ctx.Issue.Repo {
			continue
		}

		// Safely extract other fields, with fallbacks
		title, _ := res.Payload["title"].(string)
		fullText, _ := res.Payload["text"].(string)

		if title == "" {
			// Try to extract from text if title is missing (indexers often put it there)
			if strings.HasPrefix(fullText, "Title: ") {
				lines := strings.SplitN(fullText, "\n", 2)
				title = strings.TrimPrefix(lines[0], "Title: ")
			} else {
				title = "Similar Issue"
			}
		}

		url, _ := res.Payload["url"].(string)
		state, _ := res.Payload["state"].(string)
		if state == "" {
			state = "unknown"
		}
		threadType, _ := res.Payload["type"].(string)

		issue := pipeline.SimilarIssue{
			Number:     number,
			Title:      title,
			Body:       fullText,
			URL:        url,
			State:      state,
			Type:       normalizeSimilarThreadType(threadType),
			Similarity: float64(res.Score),
		}
		foundIssues = append(foundIssues, issue)
	}

	ctx.SimilarIssues = foundIssues
	ctx.Result.SimilarFound = foundIssues

	log.Printf("[similarity_search] Found %d similar issues for #%d", len(foundIssues), ctx.Issue.Number)

	return nil
}
