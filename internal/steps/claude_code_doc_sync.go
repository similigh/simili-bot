// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-03-11

package steps

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/similigh/simili-bot/internal/core/pipeline"
)

// handleDocSyncTrigger processes a PR that changes files matching the configured
// watch_paths. It builds a prompt instructing Claude Code to update documentation.
func (s *CommandHandler) handleDocSyncTrigger(ctx *pipeline.Context, changedFiles []string) error {
	cfg := ctx.Config.ClaudeCode.DocSync
	log.Printf("[command_handler] PR #%d changes watched paths — triggering Claude Code doc sync",
		ctx.Issue.Number)

	// Build the prompt with changed files and doc paths.
	prompt := fmt.Sprintf(
		"This PR modifies source files that are tracked for documentation sync.\n\n"+
			"Changed source files:\n%s\n\n"+
			"Documentation paths to update:\n%s\n\n"+
			"Please review the code changes listed above, then update the relevant "+
			"documentation files to accurately reflect the new behavior. "+
			"Only update docs that are actually affected by these changes. "+
			"Commit any documentation updates to this PR branch.",
		"- "+strings.Join(changedFiles, "\n- "),
		"- "+strings.Join(cfg.DocPaths, "\n- "))

	writeGitHubOutput("claude_code_triggered", "true")
	writeGitHubOutput("claude_code_prompt", prompt)
	writeGitHubOutput("claude_code_mode", "doc_sync")

	return pipeline.ErrSkipPipeline
}

// matchesDocSyncPaths checks if any of the changed files match the configured
// watch_paths glob patterns. Returns the list of matched files.
func matchesDocSyncPaths(changedFiles []string, watchPaths []string) []string {
	var matched []string
	for _, file := range changedFiles {
		for _, pattern := range watchPaths {
			if ok, _ := filepath.Match(pattern, file); ok {
				matched = append(matched, file)
				break
			}
			// Also try matching just the filename against the pattern
			// for patterns like "*.go"
			if ok, _ := filepath.Match(pattern, filepath.Base(file)); ok {
				matched = append(matched, file)
				break
			}
			// Handle directory-level glob: "src/api/**" → match "src/api/foo.go"
			if strings.HasSuffix(pattern, "/**") {
				prefix := strings.TrimSuffix(pattern, "/**")
				if strings.HasPrefix(file, prefix+"/") {
					matched = append(matched, file)
					break
				}
			}
		}
	}
	return matched
}
