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
	client  *github.Client
	graphql *GraphQLClient
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

// TransferIssue transfers an issue to another repository using GitHub GraphQL API.
// Requires the user to have admin/write access to both repositories.
// targetRepo should be in "owner/repo" format.
// Returns the URL of the transferred issue.
func (c *Client) TransferIssue(ctx context.Context, org, repo string, number int, targetRepo string) (string, error) {
	// Trim whitespace from input
	targetRepo = strings.TrimSpace(targetRepo)

	// Validate input format
	parts := strings.Split(targetRepo, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid targetRepo format: expected 'owner/repo', got '%s'", targetRepo)
	}

	targetOwner, targetRepoName := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if targetOwner == "" || targetRepoName == "" {
		return "", fmt.Errorf("invalid targetRepo: owner and repo cannot be empty")
	}

	// Check if GraphQL client is available
	if c.graphql == nil {
		return "", fmt.Errorf("issue transfer requires authenticated GraphQL client")
	}

	// Get issue node ID
	issueNodeID, err := c.graphql.GetIssueNodeID(ctx, org, repo, number)
	if err != nil {
		return "", fmt.Errorf("failed to get issue node ID: %w", err)
	}

	// Get target repository node ID
	targetRepoNodeID, err := c.graphql.GetRepositoryNodeID(ctx, targetOwner, targetRepoName)
	if err != nil {
		return "", fmt.Errorf("failed to get target repository node ID: %w", err)
	}

	// Execute transfer
	newURL, err := c.graphql.TransferIssue(ctx, issueNodeID, targetRepoNodeID)
	if err != nil {
		return "", fmt.Errorf("failed to transfer issue: %w", err)
	}

	return newURL, nil
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

// GetFileContent fetches the raw content of a file from a repository.
// ref can be a branch, tag, or commit SHA. If empty, the default branch is used.
func (c *Client) GetFileContent(ctx context.Context, org, repo, path, ref string) ([]byte, error) {
	opts := &github.RepositoryContentGetOptions{
		Ref: ref,
	}

	fileContent, _, _, err := c.client.Repositories.GetContents(ctx, org, repo, path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get file content for %s/%s/%s: %w", org, repo, path, err)
	}

	if fileContent == nil {
		return nil, fmt.Errorf("file content is nil")
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return nil, fmt.Errorf("failed to decode file content: %w", err)
	}

	return []byte(content), nil
}

// ListIssueEvents fetches timeline events for a specific issue.
// This includes events like transferred, closed, reopened, labeled, etc.
func (c *Client) ListIssueEvents(ctx context.Context, org, repo string, number int) ([]*github.IssueEvent, error) {
	// List all pages of events
	var allEvents []*github.IssueEvent
	opts := &github.ListOptions{
		PerPage: 100,
	}

	for {
		events, resp, err := c.client.Issues.ListIssueEvents(ctx, org, repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list issue events for #%d in %s/%s: %w", number, org, repo, err)
		}

		allEvents = append(allEvents, events...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allEvents, nil
}
