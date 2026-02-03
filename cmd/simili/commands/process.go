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
	// Parse flags is handled by cobra, ensuring cfgFile is set if provided

	// Prepare fetcher for inheritance
	// We need a temporary client for fetching config if needed
	var configToken string
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		configToken = token
	}

	fetcher := func(ref string) ([]byte, error) {
		// Parse ref: org/repo@branch:path
		org, repo, branch, path, err := config.ParseExtendsRef(ref)
		if err != nil {
			return nil, err
		}

		if configToken == "" {
			return nil, fmt.Errorf("GITHUB_TOKEN required to fetch remote config %s", ref)
		}

		ghClient := github.NewClient(context.Background(), configToken)
		return ghClient.GetFileContent(context.Background(), org, repo, path, branch)
	}

	// Load config with inheritance
	// Use cfgFile from flags if set, otherwise find default
	actualCfgPath := cfgFile
	if actualCfgPath == "" {
		actualCfgPath = config.FindConfigPath("")
	}

	var cfg *config.Config
	var err error

	if actualCfgPath != "" {
		cfg, err = config.LoadWithInheritance(actualCfgPath, fetcher)
		if err != nil {
			fmt.Printf("Warning: Failed to load config from %s: %v. Proceeding with defaults/env vars.\n", actualCfgPath, err)
			cfg = &config.Config{} // Fallback to empty config
		} else {
			if verbose {
				fmt.Printf("Loaded config from %s\n", actualCfgPath)
			}
		}
	} else {
		// No config file found
		if verbose {
			fmt.Println("No configuration file found. Using defaults and environment variables.")
		}
		cfg = &config.Config{}
	}
	// Apply defaults just in case
	// Note: applyDefaults is private in config package, ensuring config.Load* handles it.
	// Since we might have created a fresh struct, we rely on zero values and manual overrides below.

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
	stepNames := pipeline.ResolveSteps(cfg.Steps, workflow)

	// Initialize Dependencies
	// TODO: This should ideally be dependent on flags/config, potentially mocking interfaces if dry-run
	// But for now we try real clients if env vars exist

	// This is a simplified dependency setup for the CLI context
	deps := &pipeline.Dependencies{
		DryRun: dryRun,
	}

	// Initialize clients with error logging
	// Embedder
	// Check config then env
	geminiKey := cfg.Embedding.APIKey
	if geminiKey == "" {
		geminiKey = os.Getenv("GEMINI_API_KEY")
	}

	if geminiKey != "" {
		embedder, err := gemini.NewEmbedder(geminiKey)
		if err == nil {
			deps.Embedder = embedder
		} else {
			fmt.Printf("Warning: Failed to initialize Gemini embedder: %v\n", err)
		}
	} else {
		fmt.Println("Warning: No Gemini API Key found in config or GEMINI_API_KEY env var")
	}

	// Vector Store
	// Check for Qdrant env vars or config
	qURL := cfg.Qdrant.URL
	if val := os.Getenv("QDRANT_URL"); val != "" && (qURL == "" || qURL == "localhost:6334") {
		qURL = val
	}
	if qURL == "" {
		qURL = "localhost:6334" // Default
	}

	qKey := cfg.Qdrant.APIKey
	if val := os.Getenv("QDRANT_API_KEY"); val != "" && qKey == "" {
		qKey = val
	}

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
	// Re-use geminiKey resolved above
	if geminiKey != "" {
		llm, err := gemini.NewLLMClient(geminiKey)
		if err == nil {
			deps.LLMClient = llm
		} else {
			fmt.Printf("Warning: Failed to initialize Gemini LLM client: %v\n", err)
		}
	}

	defer deps.Close()

	// Check if running in CI/non-interactive environment
	isCI := os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true"

	if isCI {
		// Run pipeline directly without TUI in CI environments
		fmt.Println("[Simili-Bot] Running in CI mode (no TUI)")
		runPipeline(nil, deps, stepNames, &issue, cfg, statusChan)
		fmt.Println("[Simili-Bot] Pipeline completed")
	} else {
		// Create TUI model for interactive mode
		model := tui.NewModel(stepNames, statusChan)
		p := tea.NewProgram(model)

		// Run pipeline in a goroutine
		go func() {
			// Start processing
			runPipeline(p, deps, stepNames, &issue, cfg, statusChan)
		}()

		if _, err := p.Run(); err != nil {
			fmt.Printf("Error running TUI: %v\n", err)
			os.Exit(1)
		}
	}
}
