package commands

import (
	"testing"
	"time"

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
