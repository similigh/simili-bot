// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package steps provides the response builder step.
package steps

import (
	"fmt"
	"log"
	"strings"

	"github.com/similigh/simili-bot/internal/core/pipeline"
)

// ResponseBuilder constructs the comment to post on the issue.
type ResponseBuilder struct{}

// NewResponseBuilder creates a new response builder step.
func NewResponseBuilder() *ResponseBuilder {
	return &ResponseBuilder{}
}

// Name returns the step name.
func (s *ResponseBuilder) Name() string {
	return "response_builder"
}

// Run builds the response comment.
func (s *ResponseBuilder) Run(ctx *pipeline.Context) error {
	if len(ctx.SimilarIssues) == 0 && ctx.TransferTarget == "" {
		log.Printf("[response_builder] No similar issues or transfer target, skipping comment")
		return nil
	}

	// Build comment
	var parts []string

	// Similar issues section
	if len(ctx.SimilarIssues) > 0 {
		parts = append(parts, "## 🔍 Similar Issues Found\n")
		parts = append(parts, "I found some issues that might be related:\n")
		for _, similar := range ctx.SimilarIssues {
			parts = append(parts, fmt.Sprintf("- [#%d](%s) - %s (%.0f%% similar)",
				similar.Number, similar.URL, similar.Title, similar.Similarity*100))
		}
		parts = append(parts, "")
	}

	// Transfer notification
	if ctx.TransferTarget != "" {
		parts = append(parts, "## 📦 Transfer Suggested\n")
		parts = append(parts, fmt.Sprintf("This issue may belong in **%s**.\n", ctx.TransferTarget))
	}

	// Store the built comment in metadata for the action executor
	comment := strings.Join(parts, "\n")
	ctx.Metadata["comment"] = comment

	log.Printf("[response_builder] Built comment with %d similar issues", len(ctx.SimilarIssues))

	return nil
}
