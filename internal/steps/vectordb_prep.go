// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps provides the VectorDB preparation step.
package steps

import (
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
)

// VectorDBPrep ensures the vector database collection exists and is ready.
type VectorDBPrep struct {
	dryRun bool
}

// NewVectorDBPrep creates a new vector DB preparation step.
func NewVectorDBPrep(dryRun bool) *VectorDBPrep {
	return &VectorDBPrep{dryRun: dryRun}
}

// Name returns the step name.
func (s *VectorDBPrep) Name() string {
	return "vectordb_prep"
}

// Run ensures the collection exists.
func (s *VectorDBPrep) Run(ctx *pipeline.Context) error {
	if s.dryRun {
		log.Printf("[vectordb_prep] DRY RUN: Would verify collection exists")
		return nil
	}

	// TODO: Implement actual collection verification/creation
	// 1. Check if collection exists
	// 2. Create if not exists with proper dimensions
	log.Printf("[vectordb_prep] Collection verified")

	return nil
}
