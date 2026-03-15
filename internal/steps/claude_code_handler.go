// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-03-11

package steps

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/similigh/simili-bot/internal/core/pipeline"
)

// handleClaudeCodeTrigger processes an @simili-bot comment.
// It validates the author's association, parses the query and flags,
// and writes GitHub Actions outputs so a subsequent workflow step
// can conditionally invoke the Claude Code action.
func (s *CommandHandler) handleClaudeCodeTrigger(ctx *pipeline.Context, body string) error {
	log.Printf("[command_handler] Detected @simili-bot trigger in comment by %s on #%d",
		ctx.Issue.CommentAuthor, ctx.Issue.Number)

	// 1. Validate commenter is an authorized user (org OWNER, MEMBER, or COLLABORATOR).
	assoc := strings.ToUpper(ctx.Issue.CommentAuthorAssociation)
	if assoc != "OWNER" && assoc != "MEMBER" && assoc != "COLLABORATOR" {
		log.Printf("[command_handler] Unauthorized @simili-bot trigger by %s (association: %s), skipping",
			ctx.Issue.CommentAuthor, assoc)

		// Post a polite rejection comment.
		msg := fmt.Sprintf("> [!NOTE]\n> @%s Only repository **owners**, **members**, or **collaborators** can trigger `@simili-bot`.",
			ctx.Issue.CommentAuthor)
		ctx.Metadata["comment"] = msg

		// Let action_executor post the comment, then stop the pipeline.
		// We return nil here so action_executor runs, but set a metadata flag
		// so triage steps know to skip.
		ctx.Metadata["claude_code_unauthorized"] = true
		return nil
	}

	// 2. Parse the query: everything after "@simili-bot" (case-insensitive).
	query, modelOverride := parseClaudeCodeQuery(body)
	if query == "" {
		log.Printf("[command_handler] Empty query after @simili-bot, skipping")
		return pipeline.ErrSkipPipeline
	}

	log.Printf("[command_handler] Claude Code query: %q (model override: %q)", query, modelOverride)

	// 3. Write GitHub Actions outputs so the workflow can invoke claude-code-action.
	if err := writeGitHubOutput("claude_code_triggered", "true"); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT: %v", err)
	}
	if err := writeGitHubOutput("claude_code_query", query); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT: %v", err)
	}
	if modelOverride != "" {
		if err := writeGitHubOutput("claude_code_model_override", modelOverride); err != nil {
			log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT: %v", err)
		}
	}

	// 4. Skip the rest of the triage pipeline — this is a Claude Code request.
	return pipeline.ErrSkipPipeline
}

// parseClaudeCodeQuery extracts the user's query and optional model override
// from a comment body containing @simili-bot.
//
// Examples:
//
//	"@simili-bot fix the error handling"       → ("fix the error handling", "")
//	"@simili-bot -opus refactor this module"   → ("refactor this module", "opus")
//	"Hey @simili-bot can you fix this?"        → ("can you fix this?", "")
func parseClaudeCodeQuery(body string) (query string, modelOverride string) {
	// Find @simili-bot (case-insensitive)
	lower := strings.ToLower(body)
	idx := strings.Index(lower, "@simili-bot")
	if idx == -1 {
		return "", ""
	}

	// Extract everything after "@simili-bot"
	after := strings.TrimSpace(body[idx+len("@simili-bot"):])
	if after == "" {
		return "", ""
	}

	// Check for -opus flag
	if strings.HasPrefix(strings.ToLower(after), "-opus") {
		modelOverride = "opus"
		after = strings.TrimSpace(after[len("-opus"):])
	}

	return after, modelOverride
}

// writeGitHubOutput appends a key=value pair to the $GITHUB_OUTPUT file.
// This is how Docker-based actions communicate outputs to the workflow.
func writeGitHubOutput(key, value string) error {
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		// Not running in GitHub Actions — skip silently.
		log.Printf("[command_handler] GITHUB_OUTPUT not set, skipping output %s=%s", key, value)
		return nil
	}

	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open GITHUB_OUTPUT file: %w", err)
	}
	defer f.Close()

	// For multi-line values, use the heredoc syntax. For simple values, use key=value.
	if strings.Contains(value, "\n") {
		_, err = fmt.Fprintf(f, "%s<<EOF\n%s\nEOF\n", key, value)
	} else {
		_, err = fmt.Fprintf(f, "%s=%s\n", key, value)
	}
	return err
}
