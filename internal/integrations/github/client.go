// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package github

import (
	"context"
	"fmt"

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
	_, _, err := c.client.Issues.AddLabelsToIssue(ctx, org, repo, number, labels)
	if err != nil {
		return fmt.Errorf("failed to add labels: %w", err)
	}
	return nil
}

// TransferIssue transfers an issue to another repository.
// Note: Transferring issues via API requires the user to have admin access.
// We use the GraphQL mutation typically, but Go SDK supports Transfer (beta/preview).
// However, go-github implements it via `Issues.Transfer`.
// NOTE: GitHub's Transfer API is tricky. Issues can be transferred within org or between users.
// We need target repo name. But Transfer API takes `new_owner` and `new_name`. Not just "target repo" string.
// For Simili, we usually want to move to another repo in SAME org or DIFFERENT org.
// The `pipeline.Context` provides `TransferTarget` which is likely "org/repo".
func (c *Client) TransferIssue(ctx context.Context, org, repo string, number int, targetRepo string) error {
	// targetRepo is expected to be "owner/name" or just "name" (implies same owner).
	// We need to parse it. (TODO: Add parser helper or assume "owner/name" format)

	// Check if target is "owner/name"
	// For simplicity, let's assume valid input for now.

	// Actually go-github Issues.Transfer takes (ctx, owner, repo, number, input).
	// Input has NewOwner and NewName.

	// We'll need to implement parsing logic later or assume strict input.
	// Leaving unimplemented for now or just simple logic.
	return fmt.Errorf("transfer not implemented yet")
}
