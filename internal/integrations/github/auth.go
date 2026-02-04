// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package github

import (
	"context"
	"net/http"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

// NewClient creates a new GitHub client using the provided token.
// If token is empty, it returns an unauthenticated client.
func NewClient(ctx context.Context, token string) *Client {
	var tc *http.Client
	var graphql *GraphQLClient

	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc = oauth2.NewClient(ctx, ts)
		// Initialize GraphQL client for authenticated operations
		graphql = NewGraphQLClient(tc, token)
	}

	client := github.NewClient(tc)

	return &Client{
		client:  client,
		graphql: graphql,
	}
}
