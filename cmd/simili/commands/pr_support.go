package commands

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-github/v60/github"
	similiConfig "github.com/similigh/simili-bot/internal/core/config"
	similiGithub "github.com/similigh/simili-bot/internal/integrations/github"
)

var linkedIssuePattern = regexp.MustCompile(`(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+#(\d+)\b`)

func resolvePRCollection(cfg *similiConfig.Config, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override)
	}
	if env := strings.TrimSpace(os.Getenv("QDRANT_PR_COLLECTION")); env != "" {
		return env
	}
	if strings.TrimSpace(cfg.Qdrant.PRCollection) != "" {
		return strings.TrimSpace(cfg.Qdrant.PRCollection)
	}
	if strings.TrimSpace(cfg.Qdrant.Collection) != "" {
		return strings.TrimSpace(cfg.Qdrant.Collection) + "_prs"
	}
	return "simili_prs"
}

func listAllPullRequestFilePaths(ctx context.Context, gh *similiGithub.Client, org, repo string, number int) ([]string, error) {
	files, _, err := gh.ListPullRequestFiles(ctx, org, repo, number, &github.ListOptions{
		PerPage: 100,
	})
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(files))
	for _, f := range files {
		name := strings.TrimSpace(f.GetFilename())
		if name != "" {
			paths = append(paths, name)
		}
	}

	sort.Strings(paths)
	return paths, nil
}

func buildPRMetadataText(pr *github.PullRequest, changedFiles []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Title: %s\n\n", pr.GetTitle()))

	body := strings.TrimSpace(pr.GetBody())
	if body != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n\n", body))
	}

	sb.WriteString(fmt.Sprintf("State: %s\n", pr.GetState()))
	sb.WriteString(fmt.Sprintf("Merged: %t\n", pr.GetMerged()))
	sb.WriteString(fmt.Sprintf("Author: %s\n", pr.GetUser().GetLogin()))
	sb.WriteString(fmt.Sprintf("Base Branch: %s\n", pr.GetBase().GetRef()))
	sb.WriteString(fmt.Sprintf("Head Branch: %s\n\n", pr.GetHead().GetRef()))

	if len(changedFiles) > 0 {
		sb.WriteString("Changed Files:\n")
		for _, path := range changedFiles {
			sb.WriteString("- ")
			sb.WriteString(path)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	linked := extractLinkedIssueRefs(pr.GetBody())
	if len(linked) > 0 {
		sb.WriteString("Linked Issues:\n")
		for _, issueNum := range linked {
			sb.WriteString("- #")
			sb.WriteString(strconv.Itoa(issueNum))
			sb.WriteString("\n")
		}
	}

	return strings.TrimSpace(sb.String())
}

func extractLinkedIssueRefs(body string) []int {
	matches := linkedIssuePattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[int]struct{}, len(matches))
	result := make([]int, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		number, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if _, ok := seen[number]; ok {
			continue
		}
		seen[number] = struct{}{}
		result = append(result, number)
	}

	sort.Ints(result)
	return result
}
