// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-10
// Last Modified: 2026-02-10

package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
	"github.com/similigh/simili-bot/internal/integrations/github"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
	"github.com/similigh/simili-bot/internal/steps"
)

//go:embed static/*
var staticFiles embed.FS

// IssueRequest represents the incoming issue from the frontend
type IssueRequest struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Org    string   `json:"org"`
	Repo   string   `json:"repo"`
	Labels []string `json:"labels"`
}

// AnalysisResponse represents the response sent to the frontend
type AnalysisResponse struct {
	Success         bool                    `json:"success"`
	Error           string                  `json:"error,omitempty"`
	SimilarIssues   []pipeline.SimilarIssue `json:"similar_issues"`
	IsDuplicate     bool                    `json:"is_duplicate"`
	DuplicateOf     int                     `json:"duplicate_of"`
	DuplicateReason string                  `json:"duplicate_reason"`
	QualityScore    float64                 `json:"quality_score"`
	QualityIssues   []string                `json:"quality_issues"`
	SuggestedLabels []string                `json:"suggested_labels"`
	TransferTarget  string                  `json:"transfer_target"`
	TransferReason  string                  `json:"transfer_reason"`
}

var (
	deps     *pipeline.Dependencies
	cfg      *config.Config
	stepList []string
)

func main() {
	// Load configuration
	cfgPath := config.FindConfigPath("")
	var err error
	if cfgPath != "" {
		cfg, err = config.Load(cfgPath)
		if err != nil {
			log.Printf("Warning: Failed to load config: %v", err)
			cfg = &config.Config{}
		}
	} else {
		cfg = &config.Config{}
	}

	// Override collection from env if set
	if col := os.Getenv("QDRANT_COLLECTION"); col != "" {
		cfg.Qdrant.Collection = col
	}

	// Initialize dependencies
	deps, err = initDependencies(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize dependencies: %v", err)
	}
	defer deps.Close()

	// Define pipeline steps (exclude indexer and action_executor for web)
	stepList = []string{
		"gatekeeper",
		"similarity_search",
		"duplicate_detector",
		"quality_checker",
		"triage",
	}

	// Setup routes
	staticFS, _ := fs.Sub(staticFiles, "static")
	http.Handle("/", http.FileServer(http.FS(staticFS)))
	http.HandleFunc("/api/analyze", handleAnalyze)
	http.HandleFunc("/api/health", handleHealth)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("\n🚀 Simili Web UI running at http://localhost:%s\n", port)
	fmt.Printf("   Collection: %s\n", cfg.Qdrant.Collection)
	fmt.Println("   Press Ctrl+C to stop")

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func initDependencies(cfg *config.Config) (*pipeline.Dependencies, error) {
	deps := &pipeline.Dependencies{
		DryRun: true, // Always dry-run for web UI
	}

	// Embedder (Gemini/OpenAI auto-selected by available keys)
	embedder, err := gemini.NewEmbedder(cfg.Embedding.APIKey, cfg.Embedding.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to init embedder: %w", err)
	}
	deps.Embedder = embedder

	// Qdrant
	qURL := os.Getenv("QDRANT_URL")
	if qURL == "" {
		qURL = cfg.Qdrant.URL
	}
	qKey := os.Getenv("QDRANT_API_KEY")
	if qKey == "" {
		qKey = cfg.Qdrant.APIKey
	}

	qdrantClient, err := qdrant.NewClient(qURL, qKey)
	if err != nil {
		return nil, fmt.Errorf("failed to init qdrant: %w", err)
	}
	deps.VectorStore = qdrantClient

	// GitHub (optional)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		deps.GitHub = github.NewClient(context.Background(), token)
	}

	// LLM Client
	llm, err := gemini.NewLLMClient(cfg.Embedding.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to init LLM: %w", err)
	}
	deps.LLMClient = llm

	return deps, nil
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("Failed to encode health response: %v", err)
	}
}

func handleAnalyze(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req IssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if encErr := json.NewEncoder(w).Encode(AnalysisResponse{
			Success: false,
			Error:   "Invalid JSON: " + err.Error(),
		}); encErr != nil {
			log.Printf("Failed to encode error response: %v", encErr)
		}
		return
	}

	// Validate required fields
	if req.Title == "" {
		if err := json.NewEncoder(w).Encode(AnalysisResponse{
			Success: false,
			Error:   "Title is required",
		}); err != nil {
			log.Printf("Failed to encode error response: %v", err)
		}
		return
	}

	// Default org/repo if not provided
	if req.Org == "" {
		req.Org = "ballerina-platform"
	}
	if req.Repo == "" {
		req.Repo = "ballerina-library"
	}

	// Create pipeline issue
	issue := &pipeline.Issue{
		Org:         req.Org,
		Repo:        req.Repo,
		Number:      0, // New issue
		Title:       req.Title,
		Body:        req.Body,
		Labels:      req.Labels,
		State:       "open",
		EventType:   "issues",
		EventAction: "opened",
	}

	// Run pipeline
	result, err := runPipeline(issue)
	if err != nil {
		if encErr := json.NewEncoder(w).Encode(AnalysisResponse{
			Success: false,
			Error:   "Pipeline error: " + err.Error(),
		}); encErr != nil {
			log.Printf("Failed to encode error response: %v", encErr)
		}
		return
	}

	// Send response
	if err := json.NewEncoder(w).Encode(AnalysisResponse{
		Success:         true,
		SimilarIssues:   result.SimilarFound,
		IsDuplicate:     result.IsDuplicate,
		DuplicateOf:     result.DuplicateOf,
		DuplicateReason: result.DuplicateReason,
		QualityScore:    result.QualityScore,
		QualityIssues:   result.QualityIssues,
		SuggestedLabels: result.SuggestedLabels,
		TransferTarget:  result.TransferTarget,
		TransferReason:  result.TransferReason,
	}); err != nil {
		log.Printf("Failed to encode success response: %v", err)
	}
}

func runPipeline(issue *pipeline.Issue) (*pipeline.Result, error) {
	ctx := context.Background()
	pCtx := pipeline.NewContext(ctx, issue, cfg)

	registry := pipeline.NewRegistry()
	steps.RegisterAll(registry)

	builtPipeline, err := registry.BuildFromNames(stepList, deps)
	if err != nil {
		return nil, err
	}

	if err := builtPipeline.Run(pCtx); err != nil {
		if err != pipeline.ErrSkipPipeline {
			return nil, err
		}
	}

	return pCtx.Result, nil
}
