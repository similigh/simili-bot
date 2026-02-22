// Author: Sachindu Nethmin
// GitHub: https://github.com/Sachindu-Nethmin
// Created: 2026-02-22
// Last Modified: 2026-02-22

// Package steps provides the auto-closer logic for confirmed duplicate issues.
package steps

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	githubapi "github.com/google/go-github/v60/github"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/integrations/github"
)

// AutoCloseResult holds the summary of an auto-close run.
type AutoCloseResult struct {
	Processed    int               `json:"processed"`
	Closed       int               `json:"closed"`
	SkippedGrace int               `json:"skipped_grace_period"`
	SkippedHuman int               `json:"skipped_human_activity"`
	Errors       []string          `json:"errors,omitempty"`
	Details      []AutoCloseDetail `json:"details,omitempty"`
}

// AutoCloseDetail records the outcome for a single issue.
type AutoCloseDetail struct {
	Number int    `json:"number"`
	Action string `json:"action"` // "closed", "skipped_grace", "skipped_human", "error"
	Reason string `json:"reason,omitempty"`
}

// AutoCloser scans issues with the potential-duplicate label and closes
// those whose grace period has expired and have no human activity.
type AutoCloser struct {
	github  *github.Client
	cfg     *config.Config
	dryRun  bool
	verbose bool
}

// NewAutoCloser creates a new AutoCloser.
func NewAutoCloser(gh *github.Client, cfg *config.Config, dryRun, verbose bool) *AutoCloser {
	return &AutoCloser{
		github:  gh,
		cfg:     cfg,
		dryRun:  dryRun || cfg.AutoClose.DryRun,
		verbose: verbose,
	}
}

// Run processes all open issues with the "potential-duplicate" label.
func (ac *AutoCloser) Run(ctx context.Context, org, repo string) (*AutoCloseResult, error) {
	if ac.github == nil {
		return nil, fmt.Errorf("GitHub client is required for auto-close")
	}

	result := &AutoCloseResult{}

	// Determine grace period: CLI minutes override takes precedence
	var gracePeriod time.Duration
	if ac.cfg.AutoClose.GracePeriodMinutesOverride > 0 {
		gracePeriod = time.Duration(ac.cfg.AutoClose.GracePeriodMinutesOverride) * time.Minute
	} else {
		gracePeriodHours := ac.cfg.AutoClose.GracePeriodHours
		if gracePeriodHours <= 0 {
			gracePeriodHours = 72 // Safety default: 3 days
		}
		gracePeriod = time.Duration(gracePeriodHours) * time.Hour
	}

	if ac.verbose {
		log.Printf("[auto-closer] Grace period: %v", gracePeriod)
	}

	// Fetch open issues labeled "potential-duplicate"
	issues, err := ac.fetchPotentialDuplicates(ctx, org, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch potential-duplicate issues: %w", err)
	}

	if ac.verbose {
		log.Printf("[auto-closer] Found %d open issues with 'potential-duplicate' label", len(issues))
	}

	for _, issue := range issues {
		number := issue.GetNumber()
		result.Processed++

		detail := AutoCloseDetail{Number: number}

		// 1. Check grace period
		labeledAt, err := ac.findLabeledTime(ctx, org, repo, number, "potential-duplicate")
		if err != nil {
			detail.Action = "error"
			detail.Reason = fmt.Sprintf("failed to check label time: %v", err)
			result.Errors = append(result.Errors, fmt.Sprintf("#%d: %s", number, detail.Reason))
			result.Details = append(result.Details, detail)
			continue
		}

		if labeledAt.IsZero() {
			detail.Action = "error"
			detail.Reason = "could not find when 'potential-duplicate' label was applied"
			result.Errors = append(result.Errors, fmt.Sprintf("#%d: %s", number, detail.Reason))
			result.Details = append(result.Details, detail)
			continue
		}

		elapsed := time.Since(labeledAt)
		if elapsed < gracePeriod {
			remaining := gracePeriod - elapsed
			detail.Action = "skipped_grace"
			detail.Reason = fmt.Sprintf("grace period: %s remaining", remaining.Round(time.Hour))
			result.SkippedGrace++
			result.Details = append(result.Details, detail)
			if ac.verbose {
				log.Printf("[auto-closer] #%d: %s", number, detail.Reason)
			}
			continue
		}

		// 2. Check for human activity since the label was applied
		hasHuman, err := ac.hasHumanActivity(ctx, org, repo, number, labeledAt)
		if err != nil {
			detail.Action = "error"
			detail.Reason = fmt.Sprintf("failed to check human activity: %v", err)
			result.Errors = append(result.Errors, fmt.Sprintf("#%d: %s", number, detail.Reason))
			result.Details = append(result.Details, detail)
			continue
		}

		if hasHuman {
			detail.Action = "skipped_human"
			detail.Reason = "human activity detected after label was applied"
			result.SkippedHuman++
			result.Details = append(result.Details, detail)
			if ac.verbose {
				log.Printf("[auto-closer] #%d: %s", number, detail.Reason)
			}
			continue
		}

		// 3. Close the issue
		if ac.dryRun {
			detail.Action = "closed"
			detail.Reason = "DRY RUN: would close, swap labels, and comment"
			result.Closed++
			result.Details = append(result.Details, detail)
			log.Printf("[auto-closer] DRY RUN: would close #%d", number)
			continue
		}

		if err := ac.closeIssue(ctx, org, repo, number); err != nil {
			detail.Action = "error"
			detail.Reason = fmt.Sprintf("failed to close: %v", err)
			result.Errors = append(result.Errors, fmt.Sprintf("#%d: %s", number, detail.Reason))
			result.Details = append(result.Details, detail)
			continue
		}

		detail.Action = "closed"
		detail.Reason = "grace period expired, no human activity"
		result.Closed++
		result.Details = append(result.Details, detail)
		log.Printf("[auto-closer] Closed #%d: %s", number, detail.Reason)
	}

	return result, nil
}

// fetchPotentialDuplicates retrieves all open issues with the "potential-duplicate" label.
func (ac *AutoCloser) fetchPotentialDuplicates(ctx context.Context, org, repo string) ([]*githubapi.Issue, error) {
	var all []*githubapi.Issue
	opts := &githubapi.IssueListByRepoOptions{
		State:  "open",
		Labels: []string{"potential-duplicate"},
		ListOptions: githubapi.ListOptions{
			PerPage: 100,
		},
	}

	for {
		issues, resp, err := ac.github.ListIssues(ctx, org, repo, opts)
		if err != nil {
			return nil, err
		}
		// Filter out pull requests (GitHub API returns PRs as issues)
		for _, issue := range issues {
			if issue.PullRequestLinks == nil {
				all = append(all, issue)
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return all, nil
}

// findLabeledTime finds when the "potential-duplicate" label was most recently applied.
func (ac *AutoCloser) findLabeledTime(ctx context.Context, org, repo string, number int, label string) (time.Time, error) {
	events, err := ac.github.ListIssueEvents(ctx, org, repo, number)
	if err != nil {
		return time.Time{}, err
	}

	var latest time.Time
	for _, event := range events {
		if event.Event != nil && *event.Event == "labeled" {
			if event.Label != nil && event.Label.Name != nil && *event.Label.Name == label {
				if event.CreatedAt != nil && event.CreatedAt.Time.After(latest) {
					latest = event.CreatedAt.Time
				}
			}
		}
	}

	return latest, nil
}

// hasHumanActivity checks if any non-bot user commented on the issue after the given time.
func (ac *AutoCloser) hasHumanActivity(ctx context.Context, org, repo string, number int, since time.Time) (bool, error) {
	opts := &githubapi.IssueListCommentsOptions{
		Since: &since,
		ListOptions: githubapi.ListOptions{
			PerPage: 100,
		},
	}

	comments, _, err := ac.github.ListComments(ctx, org, repo, number, opts)
	if err != nil {
		return false, err
	}

	for _, comment := range comments {
		if comment.User == nil {
			continue
		}
		author := comment.User.GetLogin()
		if !isBotUser(author, ac.cfg.BotUsers) {
			if ac.verbose {
				log.Printf("[auto-closer] #%d: human comment by %q at %v",
					number, author, comment.CreatedAt.Time)
			}
			return true, nil
		}
	}

	return false, nil
}

// isBotUser checks if the given username matches a known bot pattern.
func isBotUser(author string, configBotUsers []string) bool {
	if strings.HasSuffix(author, "[bot]") ||
		strings.HasPrefix(author, "gh-simili") ||
		strings.EqualFold(author, "simili-bot") {
		return true
	}
	for _, u := range configBotUsers {
		if strings.EqualFold(author, u) {
			return true
		}
	}
	return false
}

// closeIssue performs the close: swap labels, post comment, close issue.
func (ac *AutoCloser) closeIssue(ctx context.Context, org, repo string, number int) error {
	// Determine effective grace period for display
	gracePeriodDisplay := ac.cfg.AutoClose.GracePeriodHours
	if gracePeriodDisplay <= 0 {
		gracePeriodDisplay = 72
	}

	// Post closing comment
	comment := fmt.Sprintf(
		"<!-- simili-bot-auto-close -->\n"+
			"### Auto-Closed as Duplicate\n\n"+
			"This issue was automatically closed because it was marked as a **potential duplicate** "+
			"and no objections were raised during the %d-hour grace period.\n\n"+
			"If this was closed in error, please reopen this issue and leave a comment explaining "+
			"why it is not a duplicate.\n\n"+
			"---\n"+
			"<sub>Generated by [Simili Bot](https://github.com/similigh/simili-bot)</sub>",
		gracePeriodDisplay,
	)

	if err := ac.github.CreateComment(ctx, org, repo, number, comment); err != nil {
		return fmt.Errorf("failed to post closing comment: %w", err)
	}

	// Swap labels: remove "potential-duplicate", add "duplicate"
	if err := ac.github.RemoveLabel(ctx, org, repo, number, "potential-duplicate"); err != nil {
		log.Printf("[auto-closer] Warning: failed to remove 'potential-duplicate' label from #%d: %v", number, err)
	}

	if err := ac.github.AddLabels(ctx, org, repo, number, []string{"duplicate"}); err != nil {
		log.Printf("[auto-closer] Warning: failed to add 'duplicate' label to #%d: %v", number, err)
	}

	// Close the issue
	if err := ac.github.CloseIssue(ctx, org, repo, number); err != nil {
		return fmt.Errorf("failed to close issue: %w", err)
	}

	return nil
}
