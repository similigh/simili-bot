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

// ExecutePipeline executes the pipeline for a single issue.
// This function can be called with silent=true to suppress status reporting,
// useful for batch processing where status updates are not desired.
func ExecutePipeline(ctx context.Context, issue *pipeline.Issue, cfg *config.Config, deps *pipeline.Dependencies, stepNames []string, silent bool) (*pipeline.Result, error) {
	pCtx := pipeline.NewContext(ctx, issue, cfg)

	registry := pipeline.NewRegistry()
	steps.RegisterAll(registry)

	// Build the actual steps
	builtSteps, err := registry.BuildFromNames(stepNames, deps)
	if err != nil {
		return nil, fmt.Errorf("error building steps: %w", err)
	}

	var finalSteps []pipeline.Step
	if silent {
		// Use steps as-is without status reporting
		finalSteps = builtSteps.Steps()
	} else {
		// Wrap steps with status reporting
		for _, step := range builtSteps.Steps() {
			finalSteps = append(finalSteps, &statusReportingStep{inner: step})
		}
	}

	finalPipeline := pipeline.New(finalSteps...)

	if err := finalPipeline.Run(pCtx); err != nil {
		// ErrSkipPipeline is not an error, just a graceful exit
		if err == pipeline.ErrSkipPipeline {
			return pCtx.Result, nil
		}
		return nil, fmt.Errorf("pipeline failed: %w", err)
	}

	return pCtx.Result, nil
}

func runPipeline(deps *pipeline.Dependencies, stepNames []string, issue *pipeline.Issue, cfg *config.Config) {
	ctx := context.Background()

	result, err := ExecutePipeline(ctx, issue, cfg, deps, stepNames, false)
	if err != nil {
		fmt.Printf("❌ Pipeline failed: %s\n", err.Error())
		return
	}

	// Marshal result to JSON and print it
	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("❌ Error marshaling result: %s\n", err.Error())
		return
	}
	fmt.Println("\n=== Pipeline Result ===")
	fmt.Println(string(resultBytes))
}
