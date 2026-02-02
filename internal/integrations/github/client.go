// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v60/github"
)

// Client wraps the GitHub API client.
type Client struct {
	client *github.Client
}

// GetIssue fetches issue details.
func (c *Client) GetIssue(ctx context.Context, org, repo string, number int) (*github.Issue, error) {
	issue, _, err := c.client.Issues.Get(ctx, org, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issue: %w", err)
	}

	return issue, nil
}

// CreateComment posts a comment on an issue.
func (c *Client) CreateComment(ctx context.Context, org, repo string, number int, body string) error {
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("comment body cannot be empty")
	}

	comment := &github.IssueComment{
		Body: github.String(body),
	}
	_, _, err := c.client.Issues.CreateComment(ctx, org, repo, number, comment)
	if err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}
	return nil
}

// AddLabels adds labels to an issue.
func (c *Client) AddLabels(ctx context.Context, org, repo string, number int, labels []string) error {
	if len(labels) == 0 {
		return fmt.Errorf("labels cannot be empty")
	}

	_, _, err := c.client.Issues.AddLabelsToIssue(ctx, org, repo, number, labels)
	if err != nil {
		return fmt.Errorf("failed to add labels: %w", err)
	}
	return nil
}

// TransferIssue transfers an issue to another repository.
// Note: Transferring issues via API requires the user to have admin access.
// targetRepo should be in "owner/repo" format.
//
// TODO: GitHub's REST API for issue transfers is complex and may require GraphQL.
// This is not yet implemented. See https://docs.github.com/en/graphql/reference/mutations#transferissue
func (c *Client) TransferIssue(ctx context.Context, org, repo string, number int, targetRepo string) error {
	// Validate input format
	parts := strings.Split(targetRepo, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid targetRepo format: expected 'owner/repo', got '%s'", targetRepo)
	}

	if parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid targetRepo: owner and repo cannot be empty")
	}

	// Transfer API is not implemented yet
	return fmt.Errorf("issue transfer not yet implemented - requires GraphQL API integration")
}

// ListIssues fetches a list of issues from the repository.
// options can be used to filter by state, labels, etc.
// If options is nil, default options are used.
func (c *Client) ListIssues(ctx context.Context, org, repo string, opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
	if opts == nil {
		opts = &github.IssueListByRepoOptions{
			State: "all",
		}
	}
	issues, resp, err := c.client.Issues.ListByRepo(ctx, org, repo, opts)
	if err != nil {
		return nil, resp, fmt.Errorf("failed to list issues for %s/%s: %w", org, repo, err)
	}
	return issues, resp, nil
}

// ListComments fetches comments for a specific issue.
func (c *Client) ListComments(ctx context.Context, org, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, *github.Response, error) {
	comments, resp, err := c.client.Issues.ListComments(ctx, org, repo, number, opts)
	if err != nil {
		return nil, resp, fmt.Errorf("failed to list comments for issue #%d in %s/%s: %w", number, org, repo, err)
	}
	return comments, resp, nil
}
