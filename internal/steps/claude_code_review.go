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

// handleSecurityReviewTrigger processes a PR labeled with the security review
// trigger label (default: "security-review"). It builds a focused OWASP prompt
// instead of reviewing every PR — this avoids wasting tokens.
func (s *CommandHandler) handleSecurityReviewTrigger(ctx *pipeline.Context) error {
	log.Printf("[command_handler] PR #%d labeled for security review — triggering Claude Code",
		ctx.Issue.Number)

	prompt := fmt.Sprintf(
		"REPO: %s/%s\nPR NUMBER: %d\n\n"+
			"Perform a **targeted security review** of this PR. Focus only on:\n\n"+
			"## Critical Checks (MUST review)\n"+
			"- Hardcoded secrets, credentials, or API keys\n"+
			"- SQL injection or NoSQL injection vectors\n"+
			"- Cross-Site Scripting (XSS) in any user-facing output\n"+
			"- Broken access control or authorization bypasses\n"+
			"- Server-Side Request Forgery (SSRF)\n"+
			"- Unsafe deserialization\n\n"+
			"## Secondary Checks (review if relevant code changed)\n"+
			"- Input validation gaps\n"+
			"- Insecure cryptographic practices\n"+
			"- Sensitive data exposure in logs or error messages\n"+
			"- Race conditions\n\n"+
			"Rate each finding as: CRITICAL, HIGH, MEDIUM, or LOW.\n"+
			"If no security issues found, state that explicitly. "+
			"Do NOT pad the review with generic advice — only report concrete findings.",
		ctx.Issue.Org, ctx.Issue.Repo, ctx.Issue.Number)

	if err := writeGitHubOutput("claude_code_triggered", "true"); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_triggered: %v", err)
	}
	if err := writeGitHubOutput("claude_code_prompt", prompt); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_prompt: %v", err)
	}
	if err := writeGitHubOutput("claude_code_mode", "security_review"); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_mode: %v", err)
	}

	return pipeline.ErrSkipPipeline
}

// handleReviewChecklistTrigger processes a PR labeled with the review checklist
// trigger label (default: "review-checklist") or triggered via "@simili-bot review".
// It builds a prompt from the user-defined checklist items.
func (s *CommandHandler) handleReviewChecklistTrigger(ctx *pipeline.Context) error {
	cfg := ctx.Config.ClaudeCode.ReviewChecklist
	log.Printf("[command_handler] PR #%d triggered for review checklist",
		ctx.Issue.Number)

	if len(cfg.Items) == 0 {
		log.Printf("[command_handler] No review checklist items configured, skipping")
		return pipeline.ErrSkipPipeline
	}

	// Build checklist as markdown checkboxes.
	var checklist strings.Builder
	for _, item := range cfg.Items {
		checklist.WriteString(fmt.Sprintf("- [ ] %s\n", item))
	}

	prompt := fmt.Sprintf(
		"REPO: %s/%s\nPR NUMBER: %d\n\n"+
			"Review this PR against the following checklist:\n\n"+
			"%s\n"+
			"For each item, check if it's satisfied and comment on any that need attention. "+
			"Post a summary with the checklist results.",
		ctx.Issue.Org, ctx.Issue.Repo, ctx.Issue.Number,
		checklist.String())

	if err := writeGitHubOutput("claude_code_triggered", "true"); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_triggered: %v", err)
	}
	if err := writeGitHubOutput("claude_code_prompt", prompt); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_prompt: %v", err)
	}
	if err := writeGitHubOutput("claude_code_mode", "review_checklist"); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_mode: %v", err)
	}

	return pipeline.ErrSkipPipeline
}
