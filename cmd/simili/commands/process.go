// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-17

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	githubapi "github.com/google/go-github/v60/github"
	"github.com/spf13/cobra"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
	"github.com/similigh/simili-bot/internal/integrations/github"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
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
You can provide issue data via --issue <file>, or fetch directly from GitHub with --number.
For direct fetch, use --repo owner/name, or --org owner --repo name.
If --repo/--org are omitted, GITHUB_REPOSITORY (owner/name) is used as fallback.`,
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

		// Attempt to unmarshal directly
		if err := json.Unmarshal(data, &issue); err != nil {
			fmt.Printf("Error parsing issue JSON: %v\n", err)
			os.Exit(1)
		}

		// Check if keys were populated. If not, this might be a raw GitHub event.
		if issue.Number == 0 || issue.EventType == "" {
			var raw map[string]interface{}
			if err := json.Unmarshal(data, &raw); err == nil {
				enrichIssueFromGitHubEvent(&issue, raw)
			}
		}

		// Apply optional overrides when --issue is used
		if repoName != "" && strings.Contains(repoName, "/") {
			parts := strings.SplitN(repoName, "/", 2)
			if len(parts) == 2 {
				if strings.TrimSpace(parts[0]) != "" {
					issue.Org = strings.TrimSpace(parts[0])
				}
				if strings.TrimSpace(parts[1]) != "" {
					issue.Repo = strings.TrimSpace(parts[1])
				}
			}
		} else {
			if orgName != "" {
				issue.Org = orgName
			}
			if repoName != "" {
				issue.Repo = repoName
			}
		}
		if orgName != "" {
			issue.Org = orgName
		}
		if issueNum != 0 {
			issue.Number = issueNum
		}
	} else if issueNum != 0 {
		org, repo := resolveIssueRepo(orgName, repoName)
		if org == "" || repo == "" {
			fmt.Println("Error: when using --number, provide --repo owner/name or --org owner --repo name, or set GITHUB_REPOSITORY")
			os.Exit(1)
		}

		token := os.Getenv("TRANSFER_TOKEN")
		if token == "" {
			token = os.Getenv("GITHUB_TOKEN")
		}
		if token == "" {
			fmt.Println("Error: GITHUB_TOKEN (or TRANSFER_TOKEN) is required to fetch issue from GitHub")
			os.Exit(1)
		}

		ghClient := github.NewClient(context.Background(), token)
		ghIssue, err := ghClient.GetIssue(context.Background(), org, repo, issueNum)
		if err != nil {
			fmt.Printf("Error fetching issue from GitHub: %v\n", err)
			os.Exit(1)
		}

		issue = githubIssueToPipelineIssue(ghIssue, org, repo)
	} else {
		fmt.Println("Please provide --issue <file> or --number <issue-number>")
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("Processing Issue: %s/%s #%d\n", issue.Org, issue.Repo, issue.Number)
	}

	// Determine steps
	stepNames := pipeline.ResolveSteps(cfg.Steps, workflow)

	// Initialize Dependencies
	deps := &pipeline.Dependencies{
		DryRun: dryRun,
	}

	// Initialize clients with error logging
	// Embedder
	geminiKey := cfg.Embedding.APIKey
	if geminiKey == "" {
		geminiKey = os.Getenv("GEMINI_API_KEY")
	}

	if geminiKey != "" {
		// Use configured model or default (passed as empty string)
		embedder, err := gemini.NewEmbedder(geminiKey, cfg.Embedding.Model)
		if err == nil {
			deps.Embedder = embedder
			if verbose {
				fmt.Printf("Initialized Gemini Embedder with model: %s\n", cfg.Embedding.Model)
			}
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

	// Log Qdrant connection info (masked key)
	if verbose {
		fmt.Printf("Connecting to Qdrant at %s\n", qURL)
	}

	qdrantClient, err := qdrant.NewClient(qURL, qKey)
	if err == nil {
		deps.VectorStore = qdrantClient
	} else {
		fmt.Printf("Warning: Failed to initialize Qdrant client: %v\n", err)
	}

	// GitHub Client
	// Prioritize TRANSFER_TOKEN for cross-repo operations if available
	token := os.Getenv("TRANSFER_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	if token != "" {
		ghClient := github.NewClient(context.Background(), token)
		deps.GitHub = ghClient
	}

	// LLM Client
	llmKey := cfg.LLM.APIKey
	if llmKey == "" {
		llmKey = geminiKey // fall back to embedding key
	}
	llmModel := cfg.LLM.Model
	if envModel := os.Getenv("LLM_MODEL"); envModel != "" {
		llmModel = envModel
	}
	if llmKey != "" {
		llm, err := gemini.NewLLMClient(llmKey, llmModel)
		if err == nil {
			deps.LLMClient = llm
			if verbose {
				fmt.Printf("Initialized Gemini LLM client with model: %s\n", llmModel)
			}
		} else {
			fmt.Printf("Warning: Failed to initialize Gemini LLM client: %v\n", err)
		}
	}

	defer deps.Close()

	// Run pipeline
	fmt.Println("[Simili-Bot] Starting pipeline...")
	runPipeline(deps, stepNames, &issue, cfg)
	fmt.Println("[Simili-Bot] Pipeline completed")
}

func resolveIssueRepo(flagOrg, flagRepo string) (string, string) {
	if strings.Contains(flagRepo, "/") {
		parts := strings.SplitN(flagRepo, "/", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
	}

	if strings.TrimSpace(flagOrg) != "" && strings.TrimSpace(flagRepo) != "" {
		return strings.TrimSpace(flagOrg), strings.TrimSpace(flagRepo)
	}

	if ghRepo := strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY")); ghRepo != "" {
		parts := strings.SplitN(ghRepo, "/", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
	}

	return "", ""
}

func githubIssueToPipelineIssue(ghIssue *githubapi.Issue, org, repo string) pipeline.Issue {
	if ghIssue == nil {
		return pipeline.Issue{Org: org, Repo: repo, EventType: "issues", EventAction: "opened"}
	}

	labels := make([]string, 0, len(ghIssue.Labels))
	for _, label := range ghIssue.Labels {
		if label != nil && label.Name != nil && *label.Name != "" {
			labels = append(labels, *label.Name)
		}
	}

	createdAt := time.Time{}
	if ghIssue.CreatedAt != nil {
		createdAt = ghIssue.CreatedAt.Time
	}

	author := ""
	if ghIssue.User != nil {
		author = ghIssue.User.GetLogin()
	}

	return pipeline.Issue{
		Org:         org,
		Repo:        repo,
		Number:      ghIssue.GetNumber(),
		Title:       ghIssue.GetTitle(),
		Body:        ghIssue.GetBody(),
		State:       ghIssue.GetState(),
		Labels:      labels,
		Author:      author,
		URL:         ghIssue.GetHTMLURL(),
		CreatedAt:   createdAt,
		EventType:   "issues",
		EventAction: "opened",
	}
}

func enrichIssueFromGitHubEvent(issue *pipeline.Issue, raw map[string]interface{}) {
	if action, ok := raw["action"].(string); ok {
		issue.EventAction = action
	}

	if comm, ok := raw["comment"].(map[string]interface{}); ok {
		issue.EventType = "issue_comment"
		if body, ok := comm["body"].(string); ok {
			issue.CommentBody = body
		}
		if user, ok := comm["user"].(map[string]interface{}); ok {
			if login, ok := user["login"].(string); ok {
				issue.CommentAuthor = login
			}
		}
	}

	if iss, ok := raw["issue"].(map[string]interface{}); ok {
		populateIssuePayload(issue, iss)
		if issue.EventType == "" {
			issue.EventType = "issues"
		}
		// PR comments arrive as issue_comment with issue.pull_request set
		if issue.EventType == "issue_comment" {
			if _, hasPR := iss["pull_request"]; hasPR {
				issue.EventType = "pr_comment"
			}
		}
	}

	if pr, ok := raw["pull_request"].(map[string]interface{}); ok {
		populateIssuePayload(issue, pr)
		issue.EventType = "pull_request"
	}

	if repo, ok := raw["repository"].(map[string]interface{}); ok {
		if owner, ok := repo["owner"].(map[string]interface{}); ok {
			if login, ok := owner["login"].(string); ok {
				issue.Org = login
			}
		}
		if name, ok := repo["name"].(string); ok {
			issue.Repo = name
		}
	}

	if issue.EventType == "" {
		issue.EventType = os.Getenv("GITHUB_EVENT_NAME")
	}

	// Fall back to sender.login so the gatekeeper can filter bot-triggered
	// pull_request.edited (and similar) events, not just issue_comment events.
	// CommentAuthor is already set for issue_comment events; for all other event
	// types it remains empty and the gatekeeper's bot-author check is skipped.
	if issue.CommentAuthor == "" {
		if sender, ok := raw["sender"].(map[string]interface{}); ok {
			if login, ok := sender["login"].(string); ok {
				issue.CommentAuthor = login
			}
		}
	}
}

func populateIssuePayload(issue *pipeline.Issue, payload map[string]interface{}) {
	if num, ok := payload["number"].(float64); ok {
		issue.Number = int(num)
	}
	if title, ok := payload["title"].(string); ok {
		issue.Title = title
	}
	if body, ok := payload["body"].(string); ok {
		issue.Body = body
	}
	if state, ok := payload["state"].(string); ok {
		issue.State = state
	}
	if htmlURL, ok := payload["html_url"].(string); ok {
		issue.URL = htmlURL
	}
	if user, ok := payload["user"].(map[string]interface{}); ok {
		if login, ok := user["login"].(string); ok {
			issue.Author = login
		}
	}
	if createdAt, ok := payload["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			issue.CreatedAt = t
		}
	}

	if labels, ok := payload["labels"].([]interface{}); ok {
		parsed := make([]string, 0, len(labels))
		for _, label := range labels {
			if l, ok := label.(map[string]interface{}); ok {
				if name, ok := l["name"].(string); ok {
					parsed = append(parsed, name)
				}
			}
		}
		if len(parsed) > 0 {
			issue.Labels = parsed
		}
	}
}
