// Package state provides a GitHub API-based implementation of GitStateManager.
package state

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GitHubStateManager implements GitStateManager using the GitHub API.
// It reads/writes files to a dedicated orphan branch without local checkout.
type GitHubStateManager struct {
	token      string
	org        string
	repo       string
	branch     string
	httpClient *http.Client
}

// NewGitHubStateManager creates a new GitHub-based state manager.
func NewGitHubStateManager(token, org, repo string) *GitHubStateManager {
	return &GitHubStateManager{
		token:      token,
		org:        org,
		repo:       repo,
		branch:     DefaultStateBranch,
		httpClient: &http.Client{},
	}
}

// WithBranch sets a custom state branch name.
func (m *GitHubStateManager) WithBranch(branch string) *GitHubStateManager {
	m.branch = branch
	return m
}

// GetPendingAction retrieves a pending action for an issue.
func (m *GitHubStateManager) GetPendingAction(ctx context.Context, org, repo string, issueNumber int) (*PendingAction, error) {
	// Try transfer first, then close
	for _, actionType := range []ActionType{ActionTransfer, ActionClose} {
		path := pendingActionPath(actionType, org, repo, issueNumber)
		data, err := m.getFileContent(ctx, path)
		if err != nil {
			if isNotFoundError(err) {
				continue
			}
			return nil, err
		}
		return UnmarshalAction(data)
	}
	return nil, nil
}

// SetPendingAction stores a pending action.
func (m *GitHubStateManager) SetPendingAction(ctx context.Context, action *PendingAction) error {
	path := pendingActionPath(action.Type, action.Org, action.Repo, action.IssueNumber)
	data, err := MarshalAction(action)
	if err != nil {
		return fmt.Errorf("failed to marshal action: %w", err)
	}
	return m.putFileContent(ctx, path, data, fmt.Sprintf("Schedule %s for issue #%d", action.Type, action.IssueNumber))
}

// DeletePendingAction removes a pending action.
func (m *GitHubStateManager) DeletePendingAction(ctx context.Context, org, repo string, issueNumber int) error {
	// Try to delete from both paths (we don't know which type it is)
	for _, actionType := range []ActionType{ActionTransfer, ActionClose} {
		path := pendingActionPath(actionType, org, repo, issueNumber)
		if err := m.deleteFile(ctx, path, fmt.Sprintf("Remove pending %s for issue #%d", actionType, issueNumber)); err != nil {
			if !isNotFoundError(err) {
				return err
			}
		}
	}
	return nil
}

// ListPendingActions lists all pending actions of a given type.
func (m *GitHubStateManager) ListPendingActions(ctx context.Context, actionType ActionType) ([]*PendingAction, error) {
	basePath := fmt.Sprintf("%s/%s", PendingDir, actionType)
	files, err := m.listFilesRecursive(ctx, basePath)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	var actions []*PendingAction
	for _, file := range files {
		if !strings.HasSuffix(file, ".json") {
			continue
		}
		data, err := m.getFileContent(ctx, file)
		if err != nil {
			continue // Skip files we can't read
		}
		action, err := UnmarshalAction(data)
		if err != nil {
			continue
		}
		actions = append(actions, action)
	}

	return actions, nil
}

// getFileContent retrieves file content from the state branch.
func (m *GitHubStateManager) getFileContent(ctx context.Context, path string) ([]byte, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		m.org, m.repo, path, m.branch)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	m.setHeaders(req)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &notFoundError{path: path}
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding: %s", result.Encoding)
	}

	content := strings.ReplaceAll(result.Content, "\n", "")
	return base64.StdEncoding.DecodeString(content)
}

// putFileContent creates or updates a file in the state branch.
func (m *GitHubStateManager) putFileContent(ctx context.Context, path string, content []byte, message string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s",
		m.org, m.repo, path)

	// Check if file exists to get SHA
	var sha string
	existingContent, err := m.getFileContent(ctx, path)
	if err == nil {
		// File exists, need SHA for update
		sha = m.getFileSHA(ctx, path)
	} else if !isNotFoundError(err) {
		return err
	}
	_ = existingContent // Not needed, just checking existence

	payload := map[string]interface{}{
		"message": message,
		"content": base64.StdEncoding.EncodeToString(content),
		"branch":  m.branch,
	}
	if sha != "" {
		payload["sha"] = sha
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	m.setHeaders(req)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// deleteFile removes a file from the state branch.
func (m *GitHubStateManager) deleteFile(ctx context.Context, path, message string) error {
	sha := m.getFileSHA(ctx, path)
	if sha == "" {
		return &notFoundError{path: path}
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s",
		m.org, m.repo, path)

	payload := map[string]interface{}{
		"message": message,
		"sha":     sha,
		"branch":  m.branch,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	m.setHeaders(req)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// getFileSHA retrieves the SHA of a file (needed for updates/deletes).
func (m *GitHubStateManager) getFileSHA(ctx context.Context, path string) string {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		m.org, m.repo, path, m.branch)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}
	m.setHeaders(req)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var result struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	return result.SHA
}

// listFilesRecursive lists all files under a path.
func (m *GitHubStateManager) listFilesRecursive(ctx context.Context, path string) ([]string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		m.org, m.repo, path, m.branch)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	m.setHeaders(req)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &notFoundError{path: path}
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var items []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	var files []string
	for _, item := range items {
		if item.Type == "file" {
			files = append(files, item.Path)
		} else if item.Type == "dir" {
			subFiles, err := m.listFilesRecursive(ctx, item.Path)
			if err != nil {
				continue
			}
			files = append(files, subFiles...)
		}
	}

	return files, nil
}

// setHeaders sets the required headers for GitHub API requests.
func (m *GitHubStateManager) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+m.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")
}

// notFoundError indicates a file was not found.
type notFoundError struct {
	path string
}

func (e *notFoundError) Error() string {
	return fmt.Sprintf("file not found: %s", e.path)
}

func isNotFoundError(err error) bool {
	_, ok := err.(*notFoundError)
	return ok
}
