// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps contains the modular "Lego block" pipeline steps.
// Each step implements the pipeline.Step interface.
package steps

import (
	"log"
	"strings"
	"time"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/github"
)

// Gatekeeper checks if the issue's repository is enabled and applies cooldown logic.
type Gatekeeper struct{
	github *github.Client
}

// NewGatekeeper creates a new gatekeeper step.
func NewGatekeeper(deps *pipeline.Dependencies) *Gatekeeper {
	return &Gatekeeper{
		github: deps.GitHub,
	}
}

// Name returns the step name.
func (s *Gatekeeper) Name() string {
	return "gatekeeper"
}

// Run checks repository configuration and permissions.
func (s *Gatekeeper) Run(ctx *pipeline.Context) error {
	// Debug logging to understand what events we receive
	log.Printf("[gatekeeper] DEBUG: Issue #%d, EventType=%q, EventAction=%q, Repo=%s/%s",
		ctx.Issue.Number, ctx.Issue.EventType, ctx.Issue.EventAction, ctx.Issue.Org, ctx.Issue.Repo)

	// Early exit: skip events triggered by bot authors to prevent infinite loops.
	// This catches cases where the bot's own comment triggers a new workflow run.
	if ctx.Issue.CommentAuthor != "" {
		author := ctx.Issue.CommentAuthor
		if isBotAuthor(author, ctx.Config.BotUsers) {
			log.Printf("[gatekeeper] Skipping event from bot author %q", author)
			ctx.Result.Skipped = true
			ctx.Result.SkipReason = "event triggered by bot"
			return pipeline.ErrSkipPipeline
		}
	}

	// Skip triage for transferred issues (they were already triaged in source repo)
	if ctx.Issue.EventAction == "transferred" {
		log.Printf("[gatekeeper] Issue was transferred from another repo, skipping triage")
		ctx.Result.Skipped = true
		ctx.Result.SkipReason = "transferred from another repository"
		return pipeline.ErrSkipPipeline
	}

	// GitHub sends action="opened" to destination repo for transferred issues, not "transferred"
	// Check the GitHub API for actual transfer events to avoid false positives
	if ctx.Issue.EventAction == "opened" {
		if s.checkIfRecentlyTransferred(ctx) {
			log.Printf("[gatekeeper] Issue #%d was recently transferred (verified via API), skipping triage",
				ctx.Issue.Number)
			ctx.Result.Skipped = true
			ctx.Result.SkipReason = "recently transferred issue (verified via GitHub API)"
			return pipeline.ErrSkipPipeline
		}
	}

	// If repositories list is empty, allow all (single-repo mode)
	if len(ctx.Config.Repositories) == 0 {
		log.Printf("[gatekeeper] No repositories configured, allowing all (single-repo mode)")
		return nil
	}

	// Check if the repository is configured
	repoConfig := findRepoConfig(ctx)
	if repoConfig == nil {
		ctx.Result.Skipped = true
		ctx.Result.SkipReason = "repository not configured"
		return pipeline.ErrSkipPipeline
	}

	// Check if processing is enabled for this repo
	if !repoConfig.Enabled {
		ctx.Result.Skipped = true
		ctx.Result.SkipReason = "repository processing disabled"
		return pipeline.ErrSkipPipeline
	}

	log.Printf("[gatekeeper] Repository %s/%s is enabled, proceeding", ctx.Issue.Org, ctx.Issue.Repo)
	return nil
}

// isBotAuthor returns true if the given username matches a known bot pattern
// or is in the user-configured bot_users list.
func isBotAuthor(author string, configBotUsers []string) bool {
	// Built-in heuristics
	if strings.HasSuffix(author, "[bot]") ||
		strings.HasPrefix(author, "gh-simili") ||
		strings.EqualFold(author, "simili-bot") {
		return true
	}
	// User-configured bot users
	for _, u := range configBotUsers {
		if strings.EqualFold(author, u) {
			return true
		}
	}
	return false
}

// findRepoConfig looks up the repository configuration.
func findRepoConfig(ctx *pipeline.Context) *config.RepositoryConfig {
	for i := range ctx.Config.Repositories {
		repo := &ctx.Config.Repositories[i]
		if repo.Org == ctx.Issue.Org && repo.Repo == ctx.Issue.Repo {
			return repo
		}
	}
	return nil
}

// checkIfRecentlyTransferred checks if an issue was recently transferred by examining
// the GitHub issue timeline/events for "transferred" events within the last 2 minutes.
// Returns true if the issue was recently transferred, false otherwise.
func (s *Gatekeeper) checkIfRecentlyTransferred(ctx *pipeline.Context) bool {
	if s.github == nil {
		log.Printf("[gatekeeper] GitHub client not available, cannot check transfer events")
		return false
	}

	// Fetch issue timeline events
	events, err := s.github.ListIssueEvents(ctx.Ctx, ctx.Issue.Org, ctx.Issue.Repo, ctx.Issue.Number)
	if err != nil {
		log.Printf("[gatekeeper] Warning: Failed to fetch issue events for #%d: %v", ctx.Issue.Number, err)
		return false
	}

	// Look for recent "transferred" events (within last 2 minutes)
	cutoff := time.Now().Add(-2 * time.Minute)
	for _, event := range events {
		if event.Event != nil && *event.Event == "transferred" {
			if event.CreatedAt != nil && event.CreatedAt.Time.After(cutoff) {
				log.Printf("[gatekeeper] Found recent transfer event for issue #%d at %v",
					ctx.Issue.Number, event.CreatedAt.Time)
				return true
			}
		}
	}

	return false
}
