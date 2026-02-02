// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps provides the triage step.
package steps

import (
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
)

// Triage uses LLM to analyze and classify the issue.
type Triage struct{}

// NewTriage creates a new triage step.
func NewTriage() *Triage {
	return &Triage{}
}

// Name returns the step name.
func (s *Triage) Name() string {
	return "triage"
}

// Run analyzes the issue using LLM.
func (s *Triage) Run(ctx *pipeline.Context) error {
	// TODO: Implement triage logic
	// 1. Call LLM with issue content and similar issues
	// 2. Determine labels, quality score, duplicate status
	// 3. Store results in context

	log.Printf("[triage] Analyzing issue #%d", ctx.Issue.Number)

	// Placeholder: no triage performed
	return nil
}
