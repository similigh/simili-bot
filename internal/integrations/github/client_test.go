// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package github

import (
	"context"
	"testing"
)

func TestCreateCommentValidation(t *testing.T) {
	// Test that CreateComment rejects empty body
	client := &Client{client: nil} // nil client for validation testing

	err := client.CreateComment(context.Background(), "org", "repo", 1, "")
	if err == nil {
		t.Error("Expected error for empty comment body")
	}

	err = client.CreateComment(context.Background(), "org", "repo", 1, "   ")
	if err == nil {
		t.Error("Expected error for whitespace-only comment body")
	}
}

func TestAddLabelsValidation(t *testing.T) {
	// Test that AddLabels rejects empty labels slice
	client := &Client{client: nil} // nil client for validation testing

	err := client.AddLabels(context.Background(), "org", "repo", 1, []string{})
	if err == nil {
		t.Error("Expected error for empty labels slice")
	}

	err = client.AddLabels(context.Background(), "org", "repo", 1, nil)
	if err == nil {
		t.Error("Expected error for nil labels slice")
	}
}

func TestTransferIssueValidation(t *testing.T) {
	client := &Client{client: nil, graphql: nil} // nil client for validation testing

	tests := []struct {
		name       string
		targetRepo string
		shouldFail bool
	}{
		{"valid format", "owner/repo", false},
		{"missing slash", "ownerrepo", true},
		{"empty owner", "/repo", true},
		{"empty repo", "owner/", true},
		{"empty string", "", true},
		{"too many slashes", "owner/repo/extra", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.TransferIssue(context.Background(), "org", "repo", 1, tt.targetRepo)
			if tt.shouldFail && err == nil {
				t.Errorf("Expected error for targetRepo=%q", tt.targetRepo)
			}
			// Valid format but no graphql client should fail with "requires authenticated GraphQL client"
			if !tt.shouldFail && err != nil && err.Error() != "issue transfer requires authenticated GraphQL client" {
				t.Errorf("Expected 'requires authenticated GraphQL client' error, got: %v", err)
			}
		})
	}
}
