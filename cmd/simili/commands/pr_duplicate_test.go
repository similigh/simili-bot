package commands

import (
	"reflect"
	"testing"

	"github.com/similigh/simili-bot/internal/integrations/qdrant"
)

func TestExtractLinkedIssueRefs(t *testing.T) {
	body := "This change fixes #12 and closes #34. It also resolves #12 again."
	got := extractLinkedIssueRefs(body)
	want := []int{12, 34}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractLinkedIssueRefs() = %v, want %v", got, want)
	}
}

func TestBuildCandidateFromSearchResult_Issue(t *testing.T) {
	res := &qdrant.SearchResult{
		Score: 0.91,
		Payload: map[string]interface{}{
			"org":          "acme",
			"repo":         "core",
			"issue_number": int64(42),
			"title":        "Crash on startup",
			"text":         "Title: Crash on startup\n\nBody: ...",
			"url":          "https://example.test/issues/42",
			"state":        "open",
			"type":         "issue",
		},
	}

	candidate, ok := buildCandidateFromSearchResult(res)
	if !ok {
		t.Fatal("expected candidate to parse")
	}
	if candidate.ID != "issue:acme/core#42" {
		t.Fatalf("candidate.ID = %q", candidate.ID)
	}
	if candidate.EntityType != "issue" {
		t.Fatalf("candidate.EntityType = %q", candidate.EntityType)
	}
}

func TestBuildCandidateFromSearchResult_PR(t *testing.T) {
	res := &qdrant.SearchResult{
		Score: 0.88,
		Payload: map[string]interface{}{
			"org":       "acme",
			"repo":      "core",
			"pr_number": int64(77),
			"text":      "Title: Improve retry behavior\n\nDescription: ...",
			"url":       "https://example.test/pull/77",
			"type":      "pull_request",
		},
	}

	candidate, ok := buildCandidateFromSearchResult(res)
	if !ok {
		t.Fatal("expected candidate to parse")
	}
	if candidate.ID != "pull_request:acme/core#77" {
		t.Fatalf("candidate.ID = %q", candidate.ID)
	}
	if candidate.Title != "Improve retry behavior" {
		t.Fatalf("candidate.Title = %q", candidate.Title)
	}
}
