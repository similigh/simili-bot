// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package pipeline provides the core pipeline engine for Simili-Bot.
// It defines the Step interface and Context structure used by all pipeline steps.
package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/similigh/simili-bot/internal/core/config"
)

// ErrSkipPipeline indicates that the pipeline should stop gracefully.
// This is not an error condition, just an early exit (e.g., cooldown, disabled repo).
var ErrSkipPipeline = errors.New("skip remaining pipeline steps")

// Step defines the interface that all pipeline steps must implement.
type Step interface {
	// Name returns the unique identifier for this step.
	Name() string

	// Run executes the step's logic.
	// It should return ErrSkipPipeline to stop the pipeline gracefully,
	// or any other error to indicate failure.
	Run(ctx *Context) error
}

// Issue represents a GitHub issue being processed.
type Issue struct {
	Org    string
	Repo   string
	Number int
	Title  string
	Body   string
	State  string // "open" or "closed"
	Labels []string
	Author string
	URL    string
}

// Result holds the accumulated results from pipeline execution.
type Result struct {
	IssueNumber     int
	Skipped         bool
	SkipReason      string
	SimilarFound    []SimilarIssue
	TransferTarget  string
	Transferred     bool
	CommentPosted   bool
	Indexed         bool
	SuggestedLabels []string
	LabelsApplied   []string
	Errors          []error
}

// SimilarIssue represents an issue found to be similar.
type SimilarIssue struct {
	Number     int
	Title      string
	URL        string
	Similarity float64
	State      string
}

// Context carries data through the pipeline steps.
type Context struct {
	// Ctx is the Go context for cancellation and timeouts.
	Ctx context.Context

	// Issue is the issue being processed.
	Issue *Issue

	// Config is the loaded configuration.
	Config *config.Config

	// Result accumulates the processing results.
	Result *Result

	// SimilarIssues holds similar issues found by the similarity step.
	SimilarIssues []SimilarIssue

	// TransferTarget is the target repo if a transfer is determined.
	TransferTarget string

	// Metadata allows steps to pass arbitrary data to subsequent steps.
	Metadata map[string]interface{}
}

// NewContext creates a new pipeline context for an issue.
func NewContext(ctx context.Context, issue *Issue, cfg *config.Config) *Context {
	return &Context{
		Ctx:      ctx,
		Issue:    issue,
		Config:   cfg,
		Result:   &Result{IssueNumber: issue.Number},
		Metadata: make(map[string]interface{}),
	}
}

// Pipeline executes a sequence of steps.
type Pipeline struct {
	steps []Step
}

// New creates a new pipeline with the given steps.
func New(steps ...Step) *Pipeline {
	return &Pipeline{steps: steps}
}

// Run executes all steps in order.
// Stops on the first error (unless it's ErrSkipPipeline, which is graceful).
func (p *Pipeline) Run(ctx *Context) error {
	for _, step := range p.steps {
		if err := step.Run(ctx); err != nil {
			if errors.Is(err, ErrSkipPipeline) {
				// Graceful early exit
				return nil
			}
			return fmt.Errorf("step '%s' failed: %w", step.Name(), err)
		}
	}
	return nil
}

// AddStep appends a step to the pipeline.
func (p *Pipeline) AddStep(step Step) {
	p.steps = append(p.steps, step)
}

// Steps returns the list of steps (for introspection).
func (p *Pipeline) Steps() []Step {
	return p.steps
}
