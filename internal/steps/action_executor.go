// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps provides the action executor step.
package steps

import (
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/github"
)

// ActionExecutor executes the decided actions (posting comments, transferring, etc).
type ActionExecutor struct {
	client *github.Client
	dryRun bool
}

// NewActionExecutor creates a new action executor step.
func NewActionExecutor(deps *pipeline.Dependencies) *ActionExecutor {
	return &ActionExecutor{
		client: deps.GitHub,
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
		// TODO: Dry run log for labels if we add label logic
		return nil
	}

	if s.client == nil {
		log.Printf("[action_executor] WARNING: No GitHub client configured, skipping actions")
		return nil
	}

	// 1. Post comment
	if hasComment && comment != "" {
		err := s.client.CreateComment(ctx.Ctx, ctx.Issue.Org, ctx.Issue.Repo, ctx.Issue.Number, comment)
		if err != nil {
			log.Printf("[action_executor] Failed to post comment: %v", err)
			ctx.Result.Errors = append(ctx.Result.Errors, err.Error())
		} else {
			log.Printf("[action_executor] Posted comment on issue #%d", ctx.Issue.Number)
			ctx.Result.CommentPosted = true
		}
	}

	// 2. Transfer issue to another repository (issues only)
	if ctx.TransferTarget != "" && ctx.Issue.EventType != "pull_request" && ctx.Issue.EventType != "pr_comment" {
		newURL, err := s.client.TransferIssue(ctx.Ctx, ctx.Issue.Org, ctx.Issue.Repo, ctx.Issue.Number, ctx.TransferTarget)
		if err != nil {
			log.Printf("[action_executor] Failed to transfer issue to %s: %v", ctx.TransferTarget, err)
			ctx.Result.Errors = append(ctx.Result.Errors, err.Error())
		} else {
			log.Printf("[action_executor] Transferred issue #%d to %s (new URL: %s)", ctx.Issue.Number, ctx.TransferTarget, newURL)
			ctx.Result.TransferTarget = ctx.TransferTarget
			ctx.Result.Transferred = true
		}
	}

	// 3. Apply labels
	if len(ctx.Result.SuggestedLabels) > 0 {
		err := s.client.AddLabels(ctx.Ctx, ctx.Issue.Org, ctx.Issue.Repo, ctx.Issue.Number, ctx.Result.SuggestedLabels)
		if err != nil {
			log.Printf("[action_executor] Failed to add labels: %v", err)
			ctx.Result.Errors = append(ctx.Result.Errors, err.Error())
		} else {
			log.Printf("[action_executor] Added labels: %v", ctx.Result.SuggestedLabels)
			ctx.Result.LabelsApplied = ctx.Result.SuggestedLabels
		}
	}

	return nil
}
