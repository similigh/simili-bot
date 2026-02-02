// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps provides the similarity search step.
package steps

import (
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
)

// SimilaritySearch finds similar issues using the vector database.
type SimilaritySearch struct {
	// embedder and vectorDB would be injected dependencies
}

// NewSimilaritySearch creates a new similarity search step.
func NewSimilaritySearch() *SimilaritySearch {
	return &SimilaritySearch{}
}

// Name returns the step name.
func (s *SimilaritySearch) Name() string {
	return "similarity_search"
}

// Run searches for similar issues.
func (s *SimilaritySearch) Run(ctx *pipeline.Context) error {
	// TODO: Implement actual similarity search
	// 1. Generate embedding for the issue
	// 2. Query Qdrant for similar vectors
	// 3. Filter by similarity threshold
	// 4. Populate ctx.SimilarIssues

	log.Printf("[similarity_search] Searching for similar issues to #%d", ctx.Issue.Number)

	// Placeholder: no similar issues found
	ctx.SimilarIssues = []pipeline.SimilarIssue{}
	ctx.Result.SimilarFound = ctx.SimilarIssues

	return nil
}
