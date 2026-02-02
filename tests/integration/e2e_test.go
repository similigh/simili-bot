// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/steps"
)

// MockStep mocks the pipeline.Step interface.
// This is provided for future test scenarios where we need to mock specific steps.
// Currently, the E2E test uses real pipeline steps to verify end-to-end behavior.
type MockStep struct {
	NameFunc func() string
	RunFunc  func(ctx *pipeline.Context) error
}

func (m *MockStep) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock_step"
}

func (m *MockStep) Run(ctx *pipeline.Context) error {
	if m.RunFunc != nil {
		return m.RunFunc(ctx)
	}
	return nil
}

func TestEndToEndPipeline(t *testing.T) {
	// 1. Setup minimal config and issue
	cfg := &config.Config{
		Defaults: config.DefaultsConfig{
			SimilarityThreshold: 0.8,
			MaxSimilarToShow:    3,
		},
	}

	issue := &pipeline.Issue{
		Org:    "test-org",
		Repo:   "test-repo",
		Number: 1337,
		Title:  "Integration Test Issue",
		Body:   "This is a test issue for E2E verification.",
		State:  "open",
	}

	ctx := context.Background()
	pCtx := pipeline.NewContext(ctx, issue, cfg)

	// 2. Setup mock dependencies (we invoke "mock-clients" via DryRun for now)
	deps := &pipeline.Dependencies{
		DryRun: true,
	}

	// 3. Create pipeline using Registry
	registry := pipeline.NewRegistry()
	steps.RegisterAll(registry)

	// Use the "issue-triage" preset
	// Note: In real E2E we would want real integrations, but for CI/basic verify here, we check plumbing.
	stepNames := pipeline.ResolveSteps(nil, "issue-triage")

	p, err := registry.BuildFromNames(stepNames, deps)
	if err != nil {
		t.Fatalf("Failed to build pipeline: %v", err)
	}

	// 4. Run Pipeline
	startTime := time.Now()
	err = p.Run(pCtx)
	duration := time.Since(startTime)

	// 5. Verify Results
	if err != nil {
		t.Fatalf("Pipeline execution failed: %v", err)
	}

	t.Logf("Pipeline passed in %v", duration)
	t.Logf("Result: %+v", pCtx.Result)

	// In "DryRun" mode on "triage" step (if implemented to skip LLM or use mock),
	// we might not get suggested labels if the step requires real LLM.
	// But "gatekeeper" should have passed.

	if pCtx.Result.Skipped {
		t.Logf("Pipeline skipped: %s", pCtx.Result.SkipReason)
	} else {
		// If not skipped, check basics
		if pCtx.Result.IssueNumber != 1337 {
			t.Errorf("Expected issue number 1337, got %d", pCtx.Result.IssueNumber)
		}
	}
}
