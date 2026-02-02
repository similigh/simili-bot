// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package commands

import (
	"context"
	"encoding/json"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/steps"
	"github.com/similigh/simili-bot/internal/tui"
)

// Wrapper step to send status updates
type statusReportingStep struct {
	inner      pipeline.Step
	statusChan chan<- tui.PipelineStatusMsg
}

func (s *statusReportingStep) Name() string {
	return s.inner.Name()
}

func (s *statusReportingStep) Run(ctx *pipeline.Context) error {
	s.statusChan <- tui.PipelineStatusMsg{Step: s.Name(), Status: "started", Message: "Starting..."}

	// Artificial delay for visual effect, can be disabled via env var
	if os.Getenv("SIMILI_NO_UI_DELAY") == "" {
		time.Sleep(100 * time.Millisecond)
	}

	err := s.inner.Run(ctx)

	if err != nil {
		if err == pipeline.ErrSkipPipeline {
			s.statusChan <- tui.PipelineStatusMsg{Step: s.Name(), Status: "skipped", Message: ctx.Result.SkipReason}
			return err
		}
		s.statusChan <- tui.PipelineStatusMsg{Step: s.Name(), Status: "error", Message: err.Error()}
		return err
	}

	s.statusChan <- tui.PipelineStatusMsg{Step: s.Name(), Status: "success", Message: "Completed"}
	return nil
}

func runPipeline(p *tea.Program, deps *pipeline.Dependencies, stepNames []string, issue *pipeline.Issue, cfg *config.Config, statusChan chan tui.PipelineStatusMsg) {
	defer close(statusChan)

	ctx := context.Background()
	pCtx := pipeline.NewContext(ctx, issue, cfg)

	registry := pipeline.NewRegistry()
	steps.RegisterAll(registry)

	// Build the actual steps
	builtSteps, err := registry.BuildFromNames(stepNames, deps)
	if err != nil {
		statusChan <- tui.PipelineStatusMsg{Step: "init", Status: "error", Message: err.Error()}
		p.Send(tui.ResultMsg{Success: false, Output: err.Error()})
		return
	}

	// Wrap steps with status reporting
	var wrappedSteps []pipeline.Step
	for _, step := range builtSteps.Steps() {
		wrappedSteps = append(wrappedSteps, &statusReportingStep{inner: step, statusChan: statusChan})
	}

	finalPipeline := pipeline.New(wrappedSteps...)

	if err := finalPipeline.Run(pCtx); err != nil {
		// Error handling is done inside the wrapper mostly, but catching the final return
		p.Send(tui.ResultMsg{Success: false, Output: err.Error()})
		return
	}

	// Marshal result to JSON
	resultBytes, err := json.MarshalIndent(pCtx.Result, "", "  ")
	if err != nil {
		p.Send(tui.ResultMsg{Success: false, Output: err.Error()})
		return
	}
	p.Send(tui.ResultMsg{Success: true, Output: string(resultBytes)})
}
