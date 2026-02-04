// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package steps

import (
	"github.com/similigh/simili-bot/internal/core/pipeline"
)

// RegisterAll registers all built-in steps with the registry.
func RegisterAll(r *pipeline.Registry) {
	r.Register("gatekeeper", func(deps *pipeline.Dependencies) (pipeline.Step, error) {
		return NewGatekeeper(deps), nil
	})

	r.Register("command_handler", func(deps *pipeline.Dependencies) (pipeline.Step, error) {
		return NewCommandHandler(deps), nil
	})

	r.Register("vectordb_prep", func(deps *pipeline.Dependencies) (pipeline.Step, error) {
		return NewVectorDBPrep(deps), nil
	})

	r.Register("similarity_search", func(deps *pipeline.Dependencies) (pipeline.Step, error) {
		return NewSimilaritySearch(deps), nil
	})

	r.Register("transfer_check", func(deps *pipeline.Dependencies) (pipeline.Step, error) {
		return NewTransferCheck(deps), nil
	})

	r.Register("triage", func(deps *pipeline.Dependencies) (pipeline.Step, error) {
		return NewTriage(deps), nil
	})

	r.Register("llm_router", func(deps *pipeline.Dependencies) (pipeline.Step, error) {
		return NewLLMRouter(deps), nil
	})

	r.Register("quality_checker", func(deps *pipeline.Dependencies) (pipeline.Step, error) {
		return NewQualityChecker(deps), nil
	})

	r.Register("duplicate_detector", func(deps *pipeline.Dependencies) (pipeline.Step, error) {
		return NewDuplicateDetector(deps), nil
	})

	r.Register("response_builder", func(deps *pipeline.Dependencies) (pipeline.Step, error) {
		return NewResponseBuilder(deps), nil
	})

	r.Register("action_executor", func(deps *pipeline.Dependencies) (pipeline.Step, error) {
		return NewActionExecutor(deps), nil
	})

	r.Register("indexer", func(deps *pipeline.Dependencies) (pipeline.Step, error) {
		return NewIndexer(deps), nil
	})

	r.Register("pending_action_scheduler", func(deps *pipeline.Dependencies) (pipeline.Step, error) {
		return NewPendingActionScheduler(deps), nil
	})
}
