// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps provides the VectorDB preparation step.
package steps

import (
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
)

// VectorDBPrep ensures the vector database collection exists and is ready.
type VectorDBPrep struct {
	client qdrant.VectorStore
	embed  *gemini.Embedder
	dryRun bool
}

// NewVectorDBPrep creates a new vector DB preparation step.
func NewVectorDBPrep(deps *pipeline.Dependencies) *VectorDBPrep {
	return &VectorDBPrep{
		client: deps.VectorStore,
		embed:  deps.Embedder,
		dryRun: deps.DryRun,
	}
}

// Name returns the step name.
func (s *VectorDBPrep) Name() string {
	return "vectordb_prep"
}

// Run ensures the collection exists.
func (s *VectorDBPrep) Run(ctx *pipeline.Context) error {
	collectionName := ctx.Config.Qdrant.Collection
	dimension := ctx.Config.Embedding.Dimensions
	if s.embed != nil && s.embed.Dimensions() > 0 {
		dimension = s.embed.Dimensions()
	}

	if s.dryRun {
		log.Printf("[vectordb_prep] DRY RUN: Would verify collection '%s' exists with dimension %d", collectionName, dimension)
		return nil
	}

	if s.client == nil {
		log.Printf("[vectordb_prep] WARNING: No vector store client configured, skipping")
		return nil
	}

	err := s.client.CreateCollection(ctx.Ctx, collectionName, dimension)
	if err != nil {
		log.Printf("[vectordb_prep] Failed to ensure collection exists: %v", err)
		return err
	}

	log.Printf("[vectordb_prep] Collection '%s' verified", collectionName)

	return nil
}
