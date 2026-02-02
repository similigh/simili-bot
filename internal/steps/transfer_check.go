// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps provides the transfer check step.
package steps

import (
	"log"

	"github.com/similigh/simili-bot/internal/core/pipeline"
)

// TransferCheck evaluates if an issue should be transferred to another repository.
type TransferCheck struct{}

// NewTransferCheck creates a new transfer check step.
func NewTransferCheck() *TransferCheck {
	return &TransferCheck{}
}

// Name returns the step name.
func (s *TransferCheck) Name() string {
	return "transfer_check"
}

// Run checks if the issue should be transferred.
func (s *TransferCheck) Run(ctx *pipeline.Context) error {
	// TODO: Implement transfer logic
	// 1. Check transfer rules (labels, title patterns, body patterns)
	// 2. Optionally use LLM for intent-based routing
	// 3. Set ctx.TransferTarget if a transfer is needed

	log.Printf("[transfer_check] Checking transfer rules for issue #%d", ctx.Issue.Number)

	// Placeholder: no transfer needed
	return nil
}
