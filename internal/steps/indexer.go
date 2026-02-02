// Package steps provides the indexer step for adding issues to the vector database.
package steps

import (
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
)

// Indexer adds/updates the issue in the vector database.
type Indexer struct {
	// embedder and vectorDB would be injected dependencies
	dryRun bool
}

// NewIndexer creates a new indexer step.
func NewIndexer(dryRun bool) *Indexer {
	return &Indexer{dryRun: dryRun}
}

// Name returns the step name.
func (s *Indexer) Name() string {
	return "indexer"
}

// Run adds the issue to the vector database.
func (s *Indexer) Run(ctx *pipeline.Context) error {
	if s.dryRun {
		log.Printf("[indexer] DRY RUN: Would index issue #%d", ctx.Issue.Number)
		return nil
	}

	// TODO: Implement actual indexing logic
	// 1. Generate embedding for issue title + body
	// 2. Store in Qdrant with metadata (org, repo, number, state, labels)
	log.Printf("[indexer] Indexed issue #%d", ctx.Issue.Number)
	ctx.Result.Indexed = true

	return nil
}
