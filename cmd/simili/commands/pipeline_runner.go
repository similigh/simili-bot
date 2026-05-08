package commands

import (
	"context"
	"encoding/json"
	"errors"
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
		if errors.Is(err, pipeline.ErrSkipPipeline) {
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

	// Separate indexer from the main pipeline so it always runs,
	// even when the pipeline is gracefully skipped (e.g., gatekeeper
	// skipping transferred issues). This ensures the VDB stays current.
	var mainNames []string
	var postNames []string
	for _, name := range stepNames {
		if name == "indexer" {
			postNames = append(postNames, name)
		} else {
			mainNames = append(mainNames, name)
		}
	}

	// Build the main pipeline steps
	builtSteps, err := registry.BuildFromNames(mainNames, deps)
	if err != nil {
		return nil, fmt.Errorf("error building steps: %w", err)
	}

	var finalSteps []pipeline.Step
	if silent {
		finalSteps = builtSteps.Steps()
	} else {
		for _, step := range builtSteps.Steps() {
			finalSteps = append(finalSteps, &statusReportingStep{inner: step})
		}
	}

	mainPipeline := pipeline.New(finalSteps...)

	pipelineErr := mainPipeline.Run(pCtx)
	if pipelineErr != nil && !errors.Is(pipelineErr, pipeline.ErrSkipPipeline) {
		return nil, fmt.Errorf("pipeline failed: %w", pipelineErr)
	}

	// Always run post-pipeline steps (indexer) — even after ErrSkipPipeline.
	// The indexer handles its own skip logic (e.g., skipping if transferred).
	if len(postNames) > 0 {
		postSteps, err := registry.BuildFromNames(postNames, deps)
		if err != nil {
			return nil, fmt.Errorf("error building post-pipeline steps: %w", err)
		}
		for _, step := range postSteps.Steps() {
			var s pipeline.Step
			if silent {
				s = step
			} else {
				s = &statusReportingStep{inner: step}
			}
			if err := s.Run(pCtx); err != nil && !errors.Is(err, pipeline.ErrSkipPipeline) {
				// Log but don't fail the overall result for post-pipeline steps
				pCtx.Result.Errors = append(pCtx.Result.Errors, fmt.Sprintf("post-pipeline step '%s': %v", step.Name(), err))
			}
		}
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
