// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps contains the modular "Lego block" pipeline steps.
// Each step implements the pipeline.Step interface.
package steps

import (
	"log"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/core/pipeline"
)

// Gatekeeper checks if the issue's repository is enabled and applies cooldown logic.
type Gatekeeper struct{}

// NewGatekeeper creates a new gatekeeper step.
func NewGatekeeper(deps *pipeline.Dependencies) *Gatekeeper {
	return &Gatekeeper{}
}

// Name returns the step name.
func (s *Gatekeeper) Name() string {
	return "gatekeeper"
}

// Run checks repository configuration and permissions.
func (s *Gatekeeper) Run(ctx *pipeline.Context) error {
	// Skip triage for transferred issues (they were already triaged in source repo)
	if ctx.Issue.EventAction == "transferred" {
		log.Printf("[gatekeeper] Issue was transferred from another repo, skipping triage")
		ctx.Result.Skipped = true
		ctx.Result.SkipReason = "transferred from another repository"
		return pipeline.ErrSkipPipeline
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
