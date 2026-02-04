// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-04
// Last Modified: 2026-02-04

package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const graphQLEndpoint = "https://api.github.com/graphql"

// GraphQLClient provides access to GitHub's GraphQL API.
type GraphQLClient struct {
	httpClient *http.Client
	token      string
}

// NewGraphQLClient creates a new GraphQL client with the given token.
func NewGraphQLClient(httpClient *http.Client, token string) *GraphQLClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &GraphQLClient{
		httpClient: httpClient,
		token:      token,
	}
}

// graphQLRequest represents a GraphQL request payload.
type graphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// graphQLResponse represents a GraphQL response.
type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// execute sends a GraphQL query/mutation and returns the response data.
func (c *GraphQLClient) execute(ctx context.Context, query string, variables map[string]interface{}) (json.RawMessage, error) {
	reqBody := graphQLRequest{
		Query:     query,
		Variables: variables,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", graphQLEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Truncate response body to avoid leaking sensitive data in logs
		truncated := string(respBody)
		if len(truncated) > 200 {
			truncated = truncated[:200] + "..."
		}
		return nil, fmt.Errorf("GraphQL request failed with status %d: %s", resp.StatusCode, truncated)
	}

	var gqlResp graphQLResponse
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	return gqlResp.Data, nil
}

// GetIssueNodeID fetches the GraphQL node ID for an issue.
func (c *GraphQLClient) GetIssueNodeID(ctx context.Context, owner, repo string, number int) (string, error) {
	query := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) {
					id
				}
			}
		}
	`
	variables := map[string]interface{}{
		"owner":  owner,
		"repo":   repo,
		"number": number,
	}

	data, err := c.execute(ctx, query, variables)
	if err != nil {
		return "", err
	}

	var result struct {
		Repository struct {
			Issue struct {
				ID string `json:"id"`
			} `json:"issue"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to parse issue ID: %w", err)
	}

	if result.Repository.Issue.ID == "" {
		return "", fmt.Errorf("issue not found: %s/%s#%d", owner, repo, number)
	}

	return result.Repository.Issue.ID, nil
}

// GetRepositoryNodeID fetches the GraphQL node ID for a repository.
func (c *GraphQLClient) GetRepositoryNodeID(ctx context.Context, owner, repo string) (string, error) {
	query := `
		query($owner: String!, $repo: String!) {
			repository(owner: $owner, name: $repo) {
				id
			}
		}
	`
	variables := map[string]interface{}{
		"owner": owner,
		"repo":  repo,
	}

	data, err := c.execute(ctx, query, variables)
	if err != nil {
		return "", err
	}

	var result struct {
		Repository struct {
			ID string `json:"id"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to parse repository ID: %w", err)
	}

	if result.Repository.ID == "" {
		return "", fmt.Errorf("repository not found: %s/%s", owner, repo)
	}

	return result.Repository.ID, nil
}

// TransferIssue transfers an issue to another repository using GraphQL mutation.
// Returns the new issue URL after transfer.
func (c *GraphQLClient) TransferIssue(ctx context.Context, issueNodeID, targetRepoNodeID string) (string, error) {
	mutation := `
		mutation($issueId: ID!, $repositoryId: ID!) {
			transferIssue(input: {issueId: $issueId, repositoryId: $repositoryId}) {
				issue {
					url
					number
				}
			}
		}
	`
	variables := map[string]interface{}{
		"issueId":      issueNodeID,
		"repositoryId": targetRepoNodeID,
	}

	data, err := c.execute(ctx, mutation, variables)
	if err != nil {
		return "", err
	}

	var result struct {
		TransferIssue struct {
			Issue struct {
				URL    string `json:"url"`
				Number int    `json:"number"`
			} `json:"issue"`
		} `json:"transferIssue"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to parse transfer result: %w", err)
	}

	if result.TransferIssue.Issue.URL == "" {
		return "", fmt.Errorf("issue transfer failed: empty URL returned")
	}

	return result.TransferIssue.Issue.URL, nil
}
