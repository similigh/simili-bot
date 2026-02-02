// Package pipeline provides step registration and preset workflow building.
package pipeline

import (
	"fmt"
	"sync"
)

// Registry holds registered step factories.
// Step factories create Step instances, allowing for dependency injection.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]StepFactory
}

// StepFactory is a function that creates a Step.
// It receives dependencies (like clients, config) as parameters.
type StepFactory func(deps *Dependencies) (Step, error)

// Dependencies holds the dependencies that can be injected into steps.
type Dependencies struct {
	// Add common dependencies here as the project grows.
	// For now, this is a placeholder for future extensibility.
}

// NewRegistry creates a new step registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]StepFactory),
	}
}

// Register adds a step factory to the registry.
func (r *Registry) Register(name string, factory StepFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Get retrieves a step factory by name.
func (r *Registry) Get(name string) (StepFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.factories[name]
	return factory, ok
}

// BuildFromNames creates a pipeline from a list of step names.
func (r *Registry) BuildFromNames(names []string, deps *Dependencies) (*Pipeline, error) {
	var steps []Step
	for _, name := range names {
		factory, ok := r.Get(name)
		if !ok {
			return nil, fmt.Errorf("unknown step: %s", name)
		}
		step, err := factory(deps)
		if err != nil {
			return nil, fmt.Errorf("failed to create step '%s': %w", name, err)
		}
		steps = append(steps, step)
	}
	return New(steps...), nil
}

// Presets defines the built-in workflow presets.
var Presets = map[string][]string{
	// issue-triage: Standard issue processing workflow
	"issue-triage": {
		"gatekeeper",
		"vectordb_prep",
		"similarity_search",
		"transfer_check",
		"triage",
		"response_builder",
		"action_executor",
		"indexer",
	},

	// similarity-only: Just find similar issues, no triage or transfers
	"similarity-only": {
		"gatekeeper",
		"vectordb_prep",
		"similarity_search",
		"response_builder",
		"action_executor",
		"indexer",
	},

	// index-only: Just index issues, no processing
	"index-only": {
		"gatekeeper",
		"vectordb_prep",
		"indexer",
	},
}

// GetPreset returns the step names for a preset workflow.
func GetPreset(name string) ([]string, bool) {
	steps, ok := Presets[name]
	return steps, ok
}

// ResolveSteps determines the steps to use based on config.
// Priority: explicit steps > workflow preset > default
func ResolveSteps(explicitSteps []string, workflow string) []string {
	if len(explicitSteps) > 0 {
		return explicitSteps
	}
	if workflow != "" {
		if preset, ok := GetPreset(workflow); ok {
			return preset
		}
	}
	// Default to issue-triage
	return Presets["issue-triage"]
}
