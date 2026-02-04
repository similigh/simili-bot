// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-04

// Package steps provides the transfer check step.
package steps

import (
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/transfer"
)

// TransferCheck evaluates if an issue should be transferred to another repository.
type TransferCheck struct{}

// NewTransferCheck creates a new transfer check step.
func NewTransferCheck(deps *pipeline.Dependencies) *TransferCheck {
	return &TransferCheck{}
}

// Name returns the step name.
func (s *TransferCheck) Name() string {
	return "transfer_check"
}

// Run checks if the issue should be transferred using transfer rules.
func (s *TransferCheck) Run(ctx *pipeline.Context) error {
	// Skip if transfer is not enabled or no rules configured
	if !ctx.Config.Transfer.Enabled || len(ctx.Config.Transfer.Rules) == 0 {
		log.Printf("[transfer_check] Transfer not enabled or no rules, skipping")
		return nil
	}

	log.Printf("[transfer_check] Checking transfer rules for issue #%d", ctx.Issue.Number)

	// Create the rule matcher
	matcher := transfer.NewRuleMatcher(ctx.Config.Transfer.Rules)

	// Build issue input for matching
	input := &transfer.IssueInput{
		Title:  ctx.Issue.Title,
		Body:   ctx.Issue.Body,
		Labels: ctx.Issue.Labels,
		Author: ctx.Issue.Author,
	}

	// Evaluate rules
	result := matcher.Match(input)
	if result.Matched {
		log.Printf("[transfer_check] Issue #%d matched rule '%s', target: %s",
			ctx.Issue.Number, result.Rule.Name, result.Target)

		// Set transfer target
		ctx.TransferTarget = result.Target
		ctx.Result.TransferTarget = result.Target

		// Store metadata for downstream steps
		ctx.Metadata["transfer_rule"] = result.Rule.Name
		ctx.Metadata["skip_duplicate_detection"] = true
	} else {
		log.Printf("[transfer_check] Issue #%d did not match any transfer rules", ctx.Issue.Number)
	}

	return nil
}
