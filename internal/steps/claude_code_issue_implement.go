// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-03-11

package steps

import (
	"fmt"
	"log"
	"strings"

	"github.com/similigh/simili-bot/internal/core/pipeline"
)

// handleIssueImplementTrigger processes an issue labeled with the implementation
// trigger label (default: "implement"). It validates the author, builds a prompt
// from the issue body, and signals Claude Code to create a PR.
func (s *CommandHandler) handleIssueImplementTrigger(ctx *pipeline.Context) error {
	cfg := ctx.Config.ClaudeCode.IssueImplement
	log.Printf("[command_handler] Issue #%d labeled '%s' — triggering Claude Code implementation",
		ctx.Issue.Number, cfg.TriggerLabel)

	// Validate commenter/actor is authorized.
	assoc := strings.ToUpper(ctx.Issue.CommentAuthorAssociation)
	if assoc != "OWNER" && assoc != "MEMBER" && assoc != "COLLABORATOR" {
		log.Printf("[command_handler] Unauthorized issue implement trigger by %s (association: %s), skipping",
			ctx.Issue.CommentAuthor, assoc)
		return pipeline.ErrSkipPipeline
	}

	// Build the prompt from issue title + body.
	prompt := fmt.Sprintf(
		"Implement the following based on this issue:\n\nTitle: %s\n\nDescription:\n%s\n\n"+
			"Create the implementation, write tests if appropriate, and ensure the code compiles.",
		ctx.Issue.Title, ctx.Issue.Body)

	// Write outputs.
	if err := writeGitHubOutput("claude_code_triggered", "true"); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_triggered: %v", err)
	}
	if err := writeGitHubOutput("claude_code_prompt", prompt); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_prompt: %v", err)
	}
	if err := writeGitHubOutput("claude_code_mode", "issue_implement"); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_mode: %v", err)
	}

	return pipeline.ErrSkipPipeline
}
