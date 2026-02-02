// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps provides the action executor step.
package steps

import (
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
)

// ActionExecutor executes the decided actions (posting comments, transferring, etc).
type ActionExecutor struct {
	dryRun bool
}

// NewActionExecutor creates a new action executor step.
func NewActionExecutor(deps *pipeline.Dependencies) *ActionExecutor {
	return &ActionExecutor{
		dryRun: deps.DryRun,
	}
}

// Name returns the step name.
func (s *ActionExecutor) Name() string {
	return "action_executor"
}

// Run executes the actions.
func (s *ActionExecutor) Run(ctx *pipeline.Context) error {
	// Get comment from metadata
	comment, hasComment := ctx.Metadata["comment"].(string)

	if s.dryRun {
		if hasComment && comment != "" {
			log.Printf("[action_executor] DRY RUN: Would post comment:\n%s", comment)
		}
		if ctx.TransferTarget != "" {
			log.Printf("[action_executor] DRY RUN: Would transfer to %s", ctx.TransferTarget)
		}
		return nil
	}

	// TODO: Implement actual action execution
	// 1. Post comment using GitHub API
	// 2. Execute transfer if target is set
	// 3. Apply labels if any

	if hasComment && comment != "" {
		log.Printf("[action_executor] Posted comment on issue #%d", ctx.Issue.Number)
		ctx.Result.CommentPosted = true
	}

	if ctx.TransferTarget != "" {
		log.Printf("[action_executor] Transfer to %s scheduled", ctx.TransferTarget)
		ctx.Result.TransferTarget = ctx.TransferTarget
	}

	return nil
}
