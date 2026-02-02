// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package steps

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/similigh/simili-bot/internal/core/pipeline"
)

// PendingActionScheduler schedules actions that could not be executed immediately.
type PendingActionScheduler struct {
	stateDir string
}

// NewPendingActionScheduler creates a new PendingActionScheduler step.
func NewPendingActionScheduler(deps *pipeline.Dependencies) *PendingActionScheduler {
	// For now, we use a local directory for state. In the future, this could be a git branch.
	return &PendingActionScheduler{
		stateDir: ".simili/pending",
	}
}

// Name returns the step name.
func (s *PendingActionScheduler) Name() string {
	return "pending_action_scheduler"
}

// PendingAction represents an action that needs to be executed later.
type PendingAction struct {
	IssueNumber    int       `json:"issue_number"`
	Org            string    `json:"org"`
	Repo           string    `json:"repo"`
	TransferTarget string    `json:"transfer_target,omitempty"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// Run executes the scheduler logic.
func (s *PendingActionScheduler) Run(ctx *pipeline.Context) error {
	// Check if there was a transfer target that wasn't executed
	if ctx.TransferTarget != "" && !ctx.Result.Transferred {
		log.Printf("[pending_action_scheduler] Scheduling pending transfer for issue #%d to %s", ctx.Issue.Number, ctx.TransferTarget)

		action := PendingAction{
			IssueNumber:    ctx.Issue.Number,
			Org:            ctx.Issue.Org,
			Repo:           ctx.Issue.Repo,
			TransferTarget: ctx.TransferTarget,
			Reason:         "Transfer failed or deferred",
			CreatedAt:      time.Now(),
			ExpiresAt:      time.Now().Add(24 * time.Hour), // Expire in 24 hours
		}

		if err := s.savePendingAction(action); err != nil {
			log.Printf("[pending_action_scheduler] Failed to save pending action: %v", err)
			return err
		}

		ctx.Result.Skipped = true
		ctx.Result.SkipReason = "Action scheduled for later"
	}

	return nil
}

func (s *PendingActionScheduler) savePendingAction(action PendingAction) error {
	if err := os.MkdirAll(s.stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state dir: %w", err)
	}

	filename := fmt.Sprintf("%s/%d_%s_%s.json", s.stateDir, action.IssueNumber, action.Org, action.Repo)
	data, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal action: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write pending action file: %w", err)
	}

	return nil
}
