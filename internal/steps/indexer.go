// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps provides the indexer step for adding issues to the vector database.
package steps

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
)

// Indexer adds/updates the issue in the vector database.
type Indexer struct {
	embedder *gemini.Embedder
	store    qdrant.VectorStore
	dryRun   bool
}

// NewIndexer creates a new indexer step.
func NewIndexer(deps *pipeline.Dependencies) *Indexer {
	return &Indexer{
		embedder: deps.Embedder,
		store:    deps.VectorStore,
		dryRun:   deps.DryRun,
	}
}

// Name returns the step name.
func (s *Indexer) Name() string {
	return "indexer"
}

// Run adds the issue to the vector database.
func (s *Indexer) Run(ctx *pipeline.Context) error {
	collectionName := ctx.Config.Qdrant.Collection

	if s.dryRun {
		log.Printf("[indexer] DRY RUN: Would index issue #%d into %s", ctx.Issue.Number, collectionName)
		return nil
	}

	if s.embedder == nil || s.store == nil {
		log.Printf("[indexer] WARNING: Missing dependencies, skipping indexing")
		return nil
	}

	// Create content for embedding
	content := fmt.Sprintf("%s\n\n%s", ctx.Issue.Title, ctx.Issue.Body)

	// Generate embedding
	embedding, err := s.embedder.Embed(ctx.Ctx, content)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Generate deterministic UUID
	uniqueID := fmt.Sprintf("%s-%s-%d", ctx.Issue.Org, ctx.Issue.Repo, ctx.Issue.Number)
	uuidID := uuid.NewMD5(uuid.NameSpaceURL, []byte(uniqueID)).String()

	// Prepare point for Qdrant
	point := &qdrant.Point{
		ID:     uuidID,
		Vector: embedding,
		Payload: map[string]interface{}{
			"org":    ctx.Issue.Org,
			"repo":   ctx.Issue.Repo,
			"number": ctx.Issue.Number,
			"title":  ctx.Issue.Title,
			"url":    ctx.Issue.URL,
			"state":  ctx.Issue.State,
			"author": ctx.Issue.Author,
			"labels": ctx.Issue.Labels,
		},
	}

	// Upsert to Qdrant
	err = s.store.Upsert(collectionName, []*qdrant.Point{point})
	if err != nil {
		return fmt.Errorf("failed to index issue: %w", err)
	}

	log.Printf("[indexer] Indexed issue #%d to %s", ctx.Issue.Number, collectionName)
	ctx.Result.Indexed = true

	return nil
}
