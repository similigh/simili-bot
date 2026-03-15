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

	if err := writeGitHubOutput("claude_code_triggered", "true"); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_triggered: %v", err)
	}
	if err := writeGitHubOutput("claude_code_prompt", prompt); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_prompt: %v", err)
	}
	if err := writeGitHubOutput("claude_code_mode", "doc_sync"); err != nil {
		log.Printf("[command_handler] Warning: failed to write GITHUB_OUTPUT claude_code_mode: %v", err)
	}

	return pipeline.ErrSkipPipeline
}

// matchesDocSyncPaths checks if any of the changed files match the configured
// watch_paths glob patterns. Returns the list of matched files.
// Patterns may use ** for recursive directory matching (e.g. "src/api/**/*.go").
func matchesDocSyncPaths(changedFiles []string, watchPaths []string) []string {
	var matched []string
	for _, file := range changedFiles {
		for _, pattern := range watchPaths {
			ok, err := matchGlob(pattern, file)
			if err != nil {
				log.Printf("[doc_sync] Warning: invalid glob pattern %q for file %q: %v", pattern, file, err)
				continue
			}
			if ok {
				matched = append(matched, file)
				break
			}
			// Also try matching just the filename for single-segment patterns like "*.go".
			if !strings.Contains(pattern, "/") {
				ok, err = filepath.Match(pattern, filepath.Base(file))
				if err != nil {
					continue
				}
				if ok {
					matched = append(matched, file)
					break
				}
			}
		}
	}
	return matched
}

// matchGlob matches a file path against a glob pattern, supporting ** for
// recursive (doublestar) directory matching.
func matchGlob(pattern, file string) (bool, error) {
	if !strings.Contains(pattern, "**") {
		return filepath.Match(pattern, file)
	}
	return matchDoublestarParts(strings.Split(pattern, "/"), strings.Split(file, "/")), nil
}

// matchDoublestarParts recursively matches path segments against pattern segments,
// where "**" matches zero or more path segments.
func matchDoublestarParts(patParts, pathParts []string) bool {
	for len(patParts) > 0 {
		if patParts[0] == "**" {
			// ** matches zero or more segments: try consuming 0 to len(pathParts) segments.
			for i := 0; i <= len(pathParts); i++ {
				if matchDoublestarParts(patParts[1:], pathParts[i:]) {
					return true
				}
			}
			return false
		}
		if len(pathParts) == 0 {
			return false
		}
		ok, err := filepath.Match(patParts[0], pathParts[0])
		if err != nil || !ok {
			return false
		}
		patParts = patParts[1:]
		pathParts = pathParts[1:]
	}
	return len(pathParts) == 0
}
