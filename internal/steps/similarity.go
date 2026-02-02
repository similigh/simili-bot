// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps provides the similarity search step.
package steps

import (
	"fmt"
	"log"

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

// Run searches for similar issues.
func (s *SimilaritySearch) Run(ctx *pipeline.Context) error {
	collectionName := ctx.Config.Qdrant.Collection
	threshold := ctx.Config.Defaults.SimilarityThreshold
	limit := ctx.Config.Defaults.MaxSimilarToShow

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
		resNumberFloat, ok := res.Payload["number"].(float64)
		if !ok {
			log.Printf("[similarity_search] WARNING: Invalid number type in payload, skipping result")
			continue
		}

		// Filter out the current issue itself if it's already indexed
		resRepo, _ := res.Payload["repo"].(string)
		if int(resNumberFloat) == ctx.Issue.Number && resRepo == ctx.Issue.Repo {
			continue
		}

		// Safely extract other fields
		title, _ := res.Payload["title"].(string)
		url, _ := res.Payload["url"].(string)
		state, _ := res.Payload["state"].(string)

		issue := pipeline.SimilarIssue{
			Number:     int(resNumberFloat),
			Title:      title,
			URL:        url,
			State:      state,
			Similarity: float64(res.Score),
		}
		foundIssues = append(foundIssues, issue)
	}

	ctx.SimilarIssues = foundIssues
	ctx.Result.SimilarFound = foundIssues

	log.Printf("[similarity_search] Found %d similar issues for #%d", len(foundIssues), ctx.Issue.Number)

	return nil
}
