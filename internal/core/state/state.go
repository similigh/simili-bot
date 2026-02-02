// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package state manages persistent state using a dedicated Git branch.
// It uses the GitHub API to read/write files without local checkout.
package state

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	// DefaultStateBranch is the name of the orphan branch used for state.
	DefaultStateBranch = "simili-state"

	// PendingDir is the directory for pending actions.
	PendingDir = "pending"
)

// ActionType defines the type of pending action.
type ActionType string

const (
	ActionTransfer ActionType = "transfer"
	ActionClose    ActionType = "close"
)

// PendingAction represents a scheduled action stored in the state branch.
type PendingAction struct {
	Type        ActionType        `json:"type"`
	Org         string            `json:"org"`
	Repo        string            `json:"repo"`
	IssueNumber int               `json:"issue_number"`
	Target      string            `json:"target"` // target repo for transfer, or original issue URL for close
	ScheduledAt time.Time         `json:"scheduled_at"`
	ExpiresAt   time.Time         `json:"expires_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// IsExpired checks if the action has expired.
func (a *PendingAction) IsExpired() bool {
	return time.Now().After(a.ExpiresAt)
}

// GitStateManager defines the interface for state operations.
// This allows for different implementations (GitHub API, local git, etc.).
type GitStateManager interface {
	// GetPendingAction retrieves a pending action for an issue.
	// Returns nil, nil if no action exists.
	GetPendingAction(ctx context.Context, org, repo string, issueNumber int) (*PendingAction, error)

	// SetPendingAction stores a pending action for an issue.
	SetPendingAction(ctx context.Context, action *PendingAction) error

	// DeletePendingAction removes a pending action for an issue.
	DeletePendingAction(ctx context.Context, org, repo string, issueNumber int) error

	// ListPendingActions lists all pending actions (optionally filtered by type).
	ListPendingActions(ctx context.Context, actionType ActionType) ([]*PendingAction, error)
}

// pendingActionPath returns the path for a pending action file.
func pendingActionPath(actionType ActionType, org, repo string, issueNumber int) string {
	return fmt.Sprintf("%s/%s/%s/%s/%d.json", PendingDir, actionType, org, repo, issueNumber)
}

// MarshalAction serializes a pending action to JSON.
func MarshalAction(action *PendingAction) ([]byte, error) {
	return json.MarshalIndent(action, "", "  ")
}

// UnmarshalAction deserializes a pending action from JSON.
func UnmarshalAction(data []byte) (*PendingAction, error) {
	var action PendingAction
	if err := json.Unmarshal(data, &action); err != nil {
		return nil, err
	}
	return &action, nil
}
