// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
	"github.com/similigh/simili-bot/internal/integrations/github"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
	"github.com/similigh/simili-bot/internal/tui"
)

var (
	issueFile string
	dryRun    bool
	workflow  string
	repoName  string
	orgName   string
	issueNum  int
)

// processCmd represents the process command
var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Process a single issue through the pipeline",
	Long: `Process a single issue through the Simili-Bot pipeline.
You can provide the issue data via a JSON file or specify the issue number (if fetching from GitHub).`,
	Run: func(cmd *cobra.Command, args []string) {
		runProcess()
	},
}

func init() {
	rootCmd.AddCommand(processCmd)

	processCmd.Flags().StringVar(&issueFile, "issue", "", "Path to issue JSON file")
	processCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Run in dry-run mode (no side effects)")
	processCmd.Flags().StringVar(&workflow, "workflow", "issue-triage", "Workflow preset to run")
	processCmd.Flags().StringVar(&repoName, "repo", "", "Repository name (override)")
	processCmd.Flags().StringVar(&orgName, "org", "", "Organization name (override)")
	processCmd.Flags().IntVar(&issueNum, "number", 0, "Issue number (override)")
}

func runProcess() {
	// 1. Load Configuration
	cfgPath := config.FindConfigPath("")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		if cfgPath != "" {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		// If no config found, proceed with zero config
		cfg = &config.Config{}
	}

	// 2. Load Issue
	var issue pipeline.Issue
	if issueFile != "" {
		data, err := os.ReadFile(issueFile)
		if err != nil {
			fmt.Printf("Error reading issue file: %v\n", err)
			os.Exit(1)
		}
		if err := json.Unmarshal(data, &issue); err != nil {
			fmt.Printf("Error parsing issue JSON: %v\n", err)
			os.Exit(1)
		}
	} else {
		// TODO: Fetch from GitHub if not provided (Phase 9/10)
		fmt.Println("Please provide --issue <file>")
		os.Exit(1)
	}

	// Override if flags provided
	if orgName != "" {
		issue.Org = orgName
	}
	if repoName != "" {
		issue.Repo = repoName
	}
	if issueNum != 0 {
		issue.Number = issueNum
	}

	statusChan := make(chan tui.PipelineStatusMsg)

	// Determine steps
	stepNames := pipeline.ResolveSteps(nil, workflow)

	// Create TUI model
	model := tui.NewModel(stepNames, statusChan)
	p := tea.NewProgram(model)

	// Run pipeline in a goroutine
	go func() {
		// Initialize Dependencies
		// TODO: This should ideally be dependent on flags/config, potentially mocking interfaces if dry-run
		// But for now we try real clients if env vars exist

		// This is a simplified dependency setup for the CLI context
		deps := &pipeline.Dependencies{
			DryRun: dryRun,
		}

		// Initialize clients with error logging
		// Embedder
		if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
			embedder, err := gemini.NewEmbedder(apiKey)
			if err == nil {
				deps.Embedder = embedder
			} else {
				fmt.Printf("Warning: Failed to initialize Gemini embedder: %v\n", err)
			}
		}

		// Vector Store
		// Check for Qdrant env vars or config
		qURL := cfg.Qdrant.URL
		if qURL == "" {
			qURL = "localhost:6334" // Default
		}
		qKey := cfg.Qdrant.APIKey

		qdrantClient, err := qdrant.NewClient(qURL, qKey)
		if err == nil {
			deps.VectorStore = qdrantClient
		} else {
			fmt.Printf("Warning: Failed to initialize Qdrant client: %v\n", err)
		}

		// GitHub Client
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			ghClient := github.NewClient(context.Background(), token)
			deps.GitHub = ghClient
		}

		// LLM Client
		if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
			llm, err := gemini.NewLLMClient(apiKey)
			if err == nil {
				deps.LLMClient = llm
			} else {
				fmt.Printf("Warning: Failed to initialize Gemini LLM client: %v\n", err)
			}
		}

		defer deps.Close()

		// Start processing
		runPipeline(p, deps, stepNames, &issue, cfg, statusChan)
	}()

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
