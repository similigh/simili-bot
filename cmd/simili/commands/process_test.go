package commands

import (
	"testing"
	"time"

	githubapi "github.com/google/go-github/v60/github"
	"github.com/similigh/simili-bot/internal/core/pipeline"
)

func TestEnrichIssueFromGitHubEvent_PullRequest(t *testing.T) {
	issue := &pipeline.Issue{}
	raw := map[string]interface{}{
		"action": "opened",
		"pull_request": map[string]interface{}{
			"number":     float64(42),
			"title":      "feat: add PR support",
			"body":       "Implements PR pipeline support",
			"state":      "open",
			"html_url":   "https://github.com/similigh/simili-bot/pull/42",
			"created_at": "2026-02-13T00:00:00Z",
			"user": map[string]interface{}{
				"login": "contributor",
			},
			"labels": []interface{}{
				map[string]interface{}{"name": "enhancement"},
				map[string]interface{}{"name": "ai"},
			},
		},
		"repository": map[string]interface{}{
			"name": "simili-bot",
			"owner": map[string]interface{}{
				"login": "similigh",
			},
		},
	}

	enrichIssueFromGitHubEvent(issue, raw)

	if issue.EventType != "pull_request" {
		t.Fatalf("expected pull_request event type, got %q", issue.EventType)
	}
	if issue.EventAction != "opened" {
		t.Fatalf("expected opened action, got %q", issue.EventAction)
	}
	if issue.Number != 42 || issue.Org != "similigh" || issue.Repo != "simili-bot" {
		t.Fatalf("unexpected issue identity: %+v", issue)
	}
	if issue.URL == "" || issue.Author != "contributor" || issue.State != "open" {
		t.Fatalf("expected PR fields to be parsed, got %+v", issue)
	}
	if len(issue.Labels) != 2 {
		t.Fatalf("expected labels to be parsed, got %+v", issue.Labels)
	}
	if issue.CreatedAt.IsZero() || !issue.CreatedAt.Equal(time.Date(2026, 2, 13, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected created_at to be parsed, got %v", issue.CreatedAt)
	}
}

func TestEnrichIssueFromGitHubEvent_IssueComment(t *testing.T) {
	issue := &pipeline.Issue{}
	raw := map[string]interface{}{
		"comment": map[string]interface{}{
			"body": "/undo",
			"user": map[string]interface{}{"login": "maintainer"},
		},
		"issue": map[string]interface{}{
			"number": float64(15),
			"title":  "Bug report",
			"body":   "Details",
		},
	}

	enrichIssueFromGitHubEvent(issue, raw)

	if issue.EventType != "issue_comment" {
		t.Fatalf("expected issue_comment event type, got %q", issue.EventType)
	}
	if issue.CommentBody != "/undo" || issue.CommentAuthor != "maintainer" {
		t.Fatalf("expected comment data, got %+v", issue)
	}
	if issue.Number != 15 {
		t.Fatalf("expected related issue number to be parsed, got %d", issue.Number)
	}
}

func TestEnrichIssueFromGitHubEvent_PRComment(t *testing.T) {
	issue := &pipeline.Issue{}
	raw := map[string]interface{}{
		"action": "created",
		"comment": map[string]interface{}{
			"body": "Looks good!",
			"user": map[string]interface{}{"login": "reviewer"},
		},
		"issue": map[string]interface{}{
			"number":       float64(42),
			"title":        "feat: add PR support",
			"body":         "PR description",
			"pull_request": map[string]interface{}{"url": "https://api.github.com/repos/org/repo/pulls/42"},
		},
		"repository": map[string]interface{}{
			"name":  "simili-bot",
			"owner": map[string]interface{}{"login": "similigh"},
		},
	}

	enrichIssueFromGitHubEvent(issue, raw)

	if issue.EventType != "pr_comment" {
		t.Fatalf("expected pr_comment event type, got %q", issue.EventType)
	}
	if issue.CommentBody != "Looks good!" || issue.CommentAuthor != "reviewer" {
		t.Fatalf("expected comment data, got body=%q author=%q", issue.CommentBody, issue.CommentAuthor)
	}
	if issue.Number != 42 {
		t.Fatalf("expected issue number 42, got %d", issue.Number)
	}
}

func TestGithubIssueToPipelineIssue(t *testing.T) {
	createdAt := githubapi.Timestamp{Time: time.Date(2026, 2, 13, 10, 0, 0, 0, time.UTC)}
	ghIssue := &githubapi.Issue{
		Number:    githubapi.Int(17),
		Title:     githubapi.String("feat: fetch issue directly"),
		Body:      githubapi.String("Implement CLI support"),
		State:     githubapi.String("open"),
		HTMLURL:   githubapi.String("https://github.com/similigh/simili-bot/issues/17"),
		CreatedAt: &createdAt,
		User:      &githubapi.User{Login: githubapi.String("maintainer")},
		Labels: []*githubapi.Label{
			{Name: githubapi.String("enhancement")},
			{Name: githubapi.String("cli")},
		},
	}

	issue := githubIssueToPipelineIssue(ghIssue, "similigh", "simili-bot")

	if issue.Org != "similigh" || issue.Repo != "simili-bot" || issue.Number != 17 {
		t.Fatalf("unexpected issue identity: %+v", issue)
	}
	if issue.Title != "feat: fetch issue directly" || issue.Body != "Implement CLI support" {
		t.Fatalf("unexpected title/body: %+v", issue)
	}
	if issue.State != "open" || issue.Author != "maintainer" || issue.URL == "" {
		t.Fatalf("expected state/author/url parsed, got %+v", issue)
	}
	if len(issue.Labels) != 2 || issue.Labels[0] != "enhancement" || issue.Labels[1] != "cli" {
		t.Fatalf("unexpected labels: %+v", issue.Labels)
	}
	if !issue.CreatedAt.Equal(createdAt.Time) {
		t.Fatalf("expected created_at to be parsed, got %v", issue.CreatedAt)
	}
	if issue.EventType != "issues" || issue.EventAction != "opened" {
		t.Fatalf("expected issues/opened event, got %s/%s", issue.EventType, issue.EventAction)
	}
}

func TestGithubIssueToPipelineIssue_NilIssue(t *testing.T) {
	issue := githubIssueToPipelineIssue(nil, "similigh", "simili-bot")

	if issue.Org != "similigh" || issue.Repo != "simili-bot" {
		t.Fatalf("unexpected org/repo for nil issue: %+v", issue)
	}
	if issue.EventType != "issues" || issue.EventAction != "opened" {
		t.Fatalf("expected default issues/opened for nil issue, got %s/%s", issue.EventType, issue.EventAction)
	}
}
