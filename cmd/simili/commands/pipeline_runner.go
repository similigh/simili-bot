package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/steps"
)

// Wrapper step to print status updates to stdout
type statusReportingStep struct {
	inner pipeline.Step
}

func (s *statusReportingStep) Name() string {
	return s.inner.Name()
}

func (s *statusReportingStep) Run(ctx *pipeline.Context) error {
	fmt.Printf("🔄 [%s] Starting...\n", s.Name())

	// Artificial delay for visual effect, can be disabled via env var
	if os.Getenv("SIMILI_NO_DELAY") == "" {
		time.Sleep(100 * time.Millisecond)
	}

	err := s.inner.Run(ctx)

	if err != nil {
		if err == pipeline.ErrSkipPipeline {
			fmt.Printf("⏭️ [%s] Skipped: %s\n", s.Name(), ctx.Result.SkipReason)
			return err
		}
		fmt.Printf("❌ [%s] Error: %s\n", s.Name(), err.Error())
		return err
	}

	fmt.Printf("✅ [%s] Completed\n", s.Name())
	return nil
}

func runPipeline(deps *pipeline.Dependencies, stepNames []string, issue *pipeline.Issue, cfg *config.Config) {
	ctx := context.Background()
	pCtx := pipeline.NewContext(ctx, issue, cfg)

	registry := pipeline.NewRegistry()
	steps.RegisterAll(registry)

	// Build the actual steps
	builtSteps, err := registry.BuildFromNames(stepNames, deps)
	if err != nil {
		fmt.Printf("❌ [init] Error building steps: %s\n", err.Error())
		return
	}

	// Wrap steps with status reporting
	var wrappedSteps []pipeline.Step
	for _, step := range builtSteps.Steps() {
		wrappedSteps = append(wrappedSteps, &statusReportingStep{inner: step})
	}

	finalPipeline := pipeline.New(wrappedSteps...)

	if err := finalPipeline.Run(pCtx); err != nil {
		fmt.Printf("❌ Pipeline failed: %s\n", err.Error())
		return
	}

	// Marshal result to JSON and print it
	resultBytes, err := json.MarshalIndent(pCtx.Result, "", "  ")
	if err != nil {
		fmt.Printf("❌ Error marshaling result: %s\n", err.Error())
		return
	}
	fmt.Println("\n=== Pipeline Result ===")
	fmt.Println(string(resultBytes))
}
