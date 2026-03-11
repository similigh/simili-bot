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

// handleMaintenanceTrigger processes a scheduled or manual workflow_dispatch event
// to run maintenance tasks. It builds a prompt from the configured task list.
func (s *CommandHandler) handleMaintenanceTrigger(ctx *pipeline.Context) error {
	cfg := ctx.Config.ClaudeCode.Maintenance
	log.Printf("[command_handler] Maintenance run triggered")

	if len(cfg.Tasks) == 0 {
		log.Printf("[command_handler] No maintenance tasks configured, skipping")
		return pipeline.ErrSkipPipeline
	}

	// Build numbered task list.
	var tasks strings.Builder
	for i, task := range cfg.Tasks {
		tasks.WriteString(fmt.Sprintf("%d. %s\n", i+1, task))
	}

	prompt := fmt.Sprintf(
		"REPO: %s/%s\n\n"+
			"Perform the following scheduled maintenance tasks:\n\n"+
			"%s\n"+
			"Create a single issue summarizing your findings with the title "+
			"\"[Maintenance] Automated scan — %s/%s\". "+
			"Only report actionable findings, skip items with no issues.",
		ctx.Issue.Org, ctx.Issue.Repo,
		tasks.String(),
		ctx.Issue.Org, ctx.Issue.Repo)

	if err := writeGitHubOutput("claude_code_triggered", "true"); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_triggered: %v", err)
	}
	if err := writeGitHubOutput("claude_code_prompt", prompt); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_prompt: %v", err)
	}
	if err := writeGitHubOutput("claude_code_mode", "maintenance"); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_mode: %v", err)
	}

	return pipeline.ErrSkipPipeline
}
