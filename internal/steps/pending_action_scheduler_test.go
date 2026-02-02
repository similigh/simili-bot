// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package steps

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/similigh/simili-bot/internal/core/pipeline"
)

func TestPendingActionScheduler_Run(t *testing.T) {
	// Create a temporary directory for state
	tempDir, err := os.MkdirTemp("", "simili_pending_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	scheduler := &PendingActionScheduler{
		stateDir: tempDir,
	}

	// Test case: Transfer target set but not transferred
	ctx := &pipeline.Context{
		Issue: &pipeline.Issue{
			Org:    "test-org",
			Repo:   "test-repo",
			Number: 123,
		},
		TransferTarget: "target-repo",
		Result: &pipeline.Result{
			Transferred: false,
		},
	}

	err = scheduler.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !ctx.Result.Skipped {
		t.Errorf("Expected result to be skipped, got false")
	}

	// Verify file exists
	expectedFile := filepath.Join(tempDir, "123_test-org_test-repo.json")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected pending action file %s to exist", expectedFile)
	}

	// Verify content
	content, _ := os.ReadFile(expectedFile)
	var action PendingAction
	json.Unmarshal(content, &action)

	if action.IssueNumber != 123 {
		t.Errorf("Expected issue number 123, got %d", action.IssueNumber)
	}
	if action.TransferTarget != "target-repo" {
		t.Errorf("Expected transfer target 'target-repo', got %s", action.TransferTarget)
	}
}

func TestPendingActionScheduler_NoActionNeeded(t *testing.T) {
	scheduler := &PendingActionScheduler{
		stateDir: "nowhere", // Should not be used
	}

	// Test case: Transferred successfully
	ctx := &pipeline.Context{
		Issue:          &pipeline.Issue{Number: 1},
		TransferTarget: "target",
		Result: &pipeline.Result{
			Transferred: true,
		},
	}

	err := scheduler.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if ctx.Result.Skipped {
		t.Errorf("Expected not skipped")
	}

	// Test case: No transfer target
	ctx = &pipeline.Context{
		Issue:          &pipeline.Issue{Number: 2},
		TransferTarget: "",
		Result:         &pipeline.Result{},
	}
	err = scheduler.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}
