// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-10
// Last Modified: 2026-02-10

package commands

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
	"github.com/similigh/simili-bot/internal/integrations/github"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
)

var (
	batchFile            string
	batchOutFile         string
	batchFormat          string
	batchWorkers         int
	batchWorkflow        string
	batchCollection      string
	batchThreshold       float64
	batchDuplicateThresh float64
	batchTopK            int
)

// BatchJob represents a job to process in the worker pool
type BatchJob struct {
	Index int
	Issue pipeline.Issue
}

// BatchResult represents the result of processing a single issue
type BatchResult struct {
	Index  int
	Issue  pipeline.Issue
	Result *pipeline.Result
	Error  error
}

// JSONOutput represents the JSON output structure
type JSONOutput struct {
	ProcessedAt time.Time     `json:"processed_at"`
	TotalIssues int           `json:"total_issues"`
	Successful  int           `json:"successful"`
	Failed      int           `json:"failed"`
	Results     []ResultEntry `json:"results"`
}

// ResultEntry represents a single result entry in JSON output
type ResultEntry struct {
	Issue  pipeline.Issue   `json:"issue"`
	Result *pipeline.Result `json:"result,omitempty"`
	Error  string           `json:"error,omitempty"`
}

// batchCmd represents the batch command
var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Process multiple issues from a JSON file",
	Long: `Process multiple issues through the pipeline in batch mode.
This command reads issues from a JSON file, processes them through the full
pipeline with dry-run mode enabled (no GitHub writes), and outputs the results
in JSON or CSV format.

Use cases:
- Test bot logic on historical data without spamming repositories
- Generate reports for stakeholders showing similarity analysis
- Perform dry-run analysis on issues from repositories without write access
- Identify duplicates and transfer recommendations in bulk`,
	Run: runBatch,
}

func init() {
	rootCmd.AddCommand(batchCmd)

	batchCmd.Flags().StringVar(&batchFile, "file", "", "Path to JSON file containing array of issues (required)")
	batchCmd.Flags().StringVar(&batchOutFile, "out-file", "", "Output file path (stdout if not specified)")
	batchCmd.Flags().StringVar(&batchFormat, "format", "json", "Output format: json or csv")
	batchCmd.Flags().IntVar(&batchWorkers, "workers", 1, "Number of concurrent workers")
	batchCmd.Flags().StringVar(&batchWorkflow, "workflow", "issue-triage", "Workflow preset to run")
	batchCmd.Flags().StringVar(&batchCollection, "collection", "", "Override Qdrant collection name")
	batchCmd.Flags().Float64Var(&batchThreshold, "threshold", 0, "Override similarity threshold")
	batchCmd.Flags().Float64Var(&batchDuplicateThresh, "duplicate-threshold", 0, "Override duplicate confidence threshold")
	batchCmd.Flags().IntVar(&batchTopK, "top-k", 0, "Override max similar issues to show")

	if err := batchCmd.MarkFlagRequired("file"); err != nil {
		fmt.Printf("Warning: Failed to mark file flag as required: %v\n", err)
	}
}

func runBatch(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// 1. Load issues from JSON file
	if verbose {
		fmt.Printf("Loading issues from %s...\n", batchFile)
	}
	issues, err := loadIssues(batchFile)
	if err != nil {
		fmt.Printf("❌ Error loading issues: %v\n", err)
		os.Exit(1)
	}
	if verbose {
		fmt.Printf("Loaded %d issues\n", len(issues))
	}

	// 2. Load Configuration
	cfgPath := cfgFile
	if cfgPath == "" {
		cfgPath = config.FindConfigPath("")
	}

	var cfg *config.Config
	if cfgPath != "" {
		// Prepare fetcher for inheritance
		var configToken string
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			configToken = token
		}

		fetcher := func(ref string) ([]byte, error) {
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

		cfg, err = config.LoadWithInheritance(cfgPath, fetcher)
		if err != nil {
			fmt.Printf("Warning: Failed to load config from %s: %v. Using defaults.\n", cfgPath, err)
			cfg = &config.Config{}
		} else if verbose {
			fmt.Printf("Loaded config from %s\n", cfgPath)
		}
	} else {
		if verbose {
			fmt.Println("No configuration file found. Using defaults and environment variables.")
		}
		cfg = &config.Config{}
	}

	// 3. Apply configuration overrides from flags
	applyConfigOverrides(cfg)

	// 4. Determine steps (exclude indexer — batch should never write to VDB)
	stepNames := pipeline.ResolveSteps(cfg.Steps, batchWorkflow)
	filtered := make([]string, 0, len(stepNames))
	for _, name := range stepNames {
		if name == "indexer" {
			if verbose {
				fmt.Println("Skipping indexer step (batch mode does not index)")
			}
			continue
		}
		filtered = append(filtered, name)
	}
	stepNames = filtered
	if verbose {
		fmt.Printf("Pipeline steps: %v\n", stepNames)
	}

	// 5. Initialize dependencies with DryRun=true
	deps, err := initializeDependencies(cfg)
	if err != nil {
		fmt.Printf("❌ Error initializing dependencies: %v\n", err)
		os.Exit(1)
	}
	defer deps.Close()

	// CRITICAL: Force dry-run mode to prevent any GitHub writes
	deps.DryRun = true
	if verbose {
		fmt.Println("✓ Dry-run mode enabled (no GitHub writes will be performed)")
	}

	// 6. Process batch
	fmt.Printf("Processing %d issues with %d workers...\n", len(issues), batchWorkers)
	results := processBatch(ctx, issues, cfg, deps, stepNames)

	// 6.5. Resolve duplicate chains across batch results (post-processing)
	resolveDuplicateChains(results)

	// 7. Output results
	if err := outputResults(results); err != nil {
		fmt.Printf("❌ Error outputting results: %v\n", err)
		os.Exit(1)
	}

	// 8. Print summary
	successful := 0
	failed := 0
	for _, r := range results {
		if r.Error == nil {
			successful++
		} else {
			failed++
		}
	}
	fmt.Printf("\n✓ Batch processing completed: %d successful, %d failed\n", successful, failed)
}

// loadIssues reads and parses a JSON file containing an array of issues
func loadIssues(filePath string) ([]pipeline.Issue, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var issues []pipeline.Issue
	if err := json.Unmarshal(data, &issues); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(issues) == 0 {
		return nil, fmt.Errorf("no issues found in file")
	}

	// Validate required fields
	for i, issue := range issues {
		if issue.Org == "" || issue.Repo == "" || issue.Number == 0 || issue.Title == "" {
			return nil, fmt.Errorf("issue at index %d missing required fields (org, repo, number, title)", i)
		}
	}

	return issues, nil
}

// applyConfigOverrides applies command-line flag overrides to the configuration
func applyConfigOverrides(cfg *config.Config) {
	if batchCollection != "" {
		cfg.Qdrant.Collection = batchCollection
		if verbose {
			fmt.Printf("Override: collection = %s\n", batchCollection)
		}
	}

	if batchThreshold > 0 {
		cfg.Defaults.SimilarityThreshold = batchThreshold
		if verbose {
			fmt.Printf("Override: similarity_threshold = %.2f\n", batchThreshold)
		}
	}

	if batchDuplicateThresh > 0 {
		cfg.Transfer.DuplicateConfidenceThreshold = batchDuplicateThresh
		if verbose {
			fmt.Printf("Override: duplicate_confidence_threshold = %.2f\n", batchDuplicateThresh)
		}
	}

	if batchTopK > 0 {
		cfg.Defaults.MaxSimilarToShow = batchTopK
		if verbose {
			fmt.Printf("Override: max_similar_to_show = %d\n", batchTopK)
		}
	}
}

// initializeDependencies initializes all required dependencies for pipeline execution
func initializeDependencies(cfg *config.Config) (*pipeline.Dependencies, error) {
	deps := &pipeline.Dependencies{}

	// Initialize Gemini Embedder
	geminiKey := cfg.Embedding.APIKey
	if geminiKey == "" {
		geminiKey = os.Getenv("GEMINI_API_KEY")
	}
	if geminiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required (set via environment or config file)")
	}

	embedder, err := gemini.NewEmbedder(geminiKey, cfg.Embedding.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Gemini embedder: %w", err)
	}
	deps.Embedder = embedder
	if verbose {
		fmt.Printf("✓ Initialized Gemini Embedder with model: %s\n", cfg.Embedding.Model)
	}

	// Initialize Qdrant Client
	qURL := cfg.Qdrant.URL
	if val := os.Getenv("QDRANT_URL"); val != "" && (qURL == "" || qURL == "localhost:6334") {
		qURL = val
	}
	if qURL == "" {
		qURL = "localhost:6334"
	}

	qKey := cfg.Qdrant.APIKey
	if val := os.Getenv("QDRANT_API_KEY"); val != "" && qKey == "" {
		qKey = val
	}

	if verbose {
		fmt.Printf("✓ Connecting to Qdrant at %s\n", qURL)
	}

	qdrantClient, err := qdrant.NewClient(qURL, qKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Qdrant client: %w", err)
	}
	deps.VectorStore = qdrantClient

	// Initialize GitHub Client (optional)
	token := os.Getenv("TRANSFER_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token != "" {
		ghClient := github.NewClient(context.Background(), token)
		deps.GitHub = ghClient
		if verbose {
			fmt.Println("✓ Initialized GitHub client")
		}
	} else if verbose {
		fmt.Println("ℹ No GitHub token found (some steps may be limited)")
	}

	// Initialize LLM Client
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
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Gemini LLM client: %w", err)
		}
		deps.LLMClient = llm
		if verbose {
			fmt.Printf("✓ Initialized Gemini LLM client with model: %s\n", llmModel)
		}
	}

	return deps, nil
}

// processBatch processes all issues using a worker pool pattern
func processBatch(ctx context.Context, issues []pipeline.Issue, cfg *config.Config, deps *pipeline.Dependencies, stepNames []string) []BatchResult {
	jobs := make(chan BatchJob, batchWorkers)
	results := make(chan BatchResult, batchWorkers)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < batchWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				if verbose {
					fmt.Printf("[Worker %d] Processing issue #%d (%s/%s)\n", workerID, job.Issue.Number, job.Issue.Org, job.Issue.Repo)
				}

				result, err := ExecutePipeline(ctx, &job.Issue, cfg, deps, stepNames, true)

				results <- BatchResult{
					Index:  job.Index,
					Issue:  job.Issue,
					Result: result,
					Error:  err,
				}

				if verbose {
					if err != nil {
						fmt.Printf("[Worker %d] ❌ Issue #%d failed: %v\n", workerID, job.Issue.Number, err)
					} else {
						fmt.Printf("[Worker %d] ✓ Issue #%d completed\n", workerID, job.Issue.Number)
					}
				}
			}
		}(i)
	}

	// Send jobs
	go func() {
		for i, issue := range issues {
			jobs <- BatchJob{Index: i, Issue: issue}
		}
		close(jobs)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Gather results in order
	resultMap := make(map[int]BatchResult)
	for result := range results {
		resultMap[result.Index] = result
	}

	orderedResults := make([]BatchResult, len(issues))
	for i := range issues {
		orderedResults[i] = resultMap[i]
	}

	return orderedResults
}

// outputResults formats and writes results to the specified output
func outputResults(results []BatchResult) error {
	var data []byte
	var err error

	// Determine format
	format := batchFormat
	if format == "" && batchOutFile != "" {
		// Infer from file extension
		ext := strings.ToLower(filepath.Ext(batchOutFile))
		if ext == ".csv" {
			format = "csv"
		} else {
			format = "json"
		}
	}
	if format == "" {
		format = "json"
	}

	// Format output
	switch format {
	case "csv":
		data, err = formatCSV(results)
	case "json":
		data, err = formatJSON(results)
	default:
		return fmt.Errorf("unsupported format: %s (use json or csv)", format)
	}

	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Write output
	if batchOutFile != "" {
		if err := os.WriteFile(batchOutFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("✓ Results written to %s\n", batchOutFile)
	} else {
		fmt.Println("\n=== Batch Results ===")
		fmt.Println(string(data))
	}

	return nil
}

// formatJSON formats results as JSON
func formatJSON(results []BatchResult) ([]byte, error) {
	successful := 0
	failed := 0
	entries := make([]ResultEntry, len(results))

	for i, r := range results {
		entry := ResultEntry{
			Issue:  r.Issue,
			Result: r.Result,
		}
		if r.Error != nil {
			entry.Error = r.Error.Error()
			failed++
		} else {
			successful++
		}
		entries[i] = entry
	}

	output := JSONOutput{
		ProcessedAt: time.Now(),
		TotalIssues: len(results),
		Successful:  successful,
		Failed:      failed,
		Results:     entries,
	}

	return json.MarshalIndent(output, "", "  ")
}

// formatCSV formats results as CSV
func formatCSV(results []BatchResult) ([]byte, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{
		"issue_number",
		"org",
		"repo",
		"title",
		"author",
		"state",
		"skipped",
		"skip_reason",
		"similar_count",
		"top_similar_number",
		"top_similar_score",
		"is_duplicate",
		"duplicate_of",
		"duplicate_confidence",
		"duplicate_reason",
		"transfer_target",
		"transfer_confidence",
		"transfer_reason",
		"quality_score",
		"suggested_labels",
		"error",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Write rows
	for _, r := range results {
		row := make([]string, len(header))
		row[0] = strconv.Itoa(r.Issue.Number)
		row[1] = r.Issue.Org
		row[2] = r.Issue.Repo
		row[3] = r.Issue.Title
		row[4] = r.Issue.Author
		row[5] = r.Issue.State

		if r.Error != nil {
			row[20] = r.Error.Error()
		} else if r.Result != nil {
			row[6] = strconv.FormatBool(r.Result.Skipped)
			row[7] = r.Result.SkipReason
			row[8] = strconv.Itoa(len(r.Result.SimilarFound))

			if len(r.Result.SimilarFound) > 0 {
				row[9] = strconv.Itoa(r.Result.SimilarFound[0].Number)
				row[10] = fmt.Sprintf("%.4f", r.Result.SimilarFound[0].Similarity)
			} else {
				row[9] = "0"
				row[10] = "0.0000"
			}

			row[11] = strconv.FormatBool(r.Result.IsDuplicate)
			row[12] = strconv.Itoa(r.Result.DuplicateOf)
			row[13] = fmt.Sprintf("%.4f", r.Result.DuplicateConfidence)
			row[14] = r.Result.DuplicateReason
			row[15] = r.Result.TransferTarget
			row[16] = fmt.Sprintf("%.4f", r.Result.TransferConfidence)
			row[17] = r.Result.TransferReason
			row[18] = fmt.Sprintf("%.4f", r.Result.QualityScore)
			row[19] = strings.Join(r.Result.SuggestedLabels, ";")
		}

		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return []byte(buf.String()), nil
}

// resolveDuplicateChains resolves transitive duplicate relationships.
// If issue A is duplicate of B, and B is duplicate of C, this updates A to point to C.
func resolveDuplicateChains(results []BatchResult) {
	// Build duplicate map: issue number -> duplicate_of
	duplicateMap := make(map[int]int)
	for _, r := range results {
		if r.Result != nil && r.Result.IsDuplicate && r.Result.DuplicateOf != 0 {
			duplicateMap[r.Issue.Number] = r.Result.DuplicateOf
		}
	}

	// If no duplicates, nothing to resolve
	if len(duplicateMap) == 0 {
		return
	}

	// Resolve chains for each duplicate
	chainsResolved := 0
	for i := range results {
		if results[i].Result != nil && results[i].Result.IsDuplicate {
			originalDup := results[i].Result.DuplicateOf
			rootIssue := findDuplicateRoot(originalDup, duplicateMap)

			if rootIssue != originalDup {
				results[i].Result.DuplicateOf = rootIssue
				chainsResolved++
				if verbose {
					fmt.Printf("  Issue #%d: resolved chain %d -> %d\n",
						results[i].Issue.Number, originalDup, rootIssue)
				}
			}
		}
	}

	if verbose && chainsResolved > 0 {
		fmt.Printf("✓ Resolved %d duplicate chains\n", chainsResolved)
	}
}

// findDuplicateRoot follows the duplicate chain to find the root issue.
// Returns the root issue number, or the original if no chain exists.
func findDuplicateRoot(issueNum int, duplicateMap map[int]int) int {
	visited := make(map[int]bool)
	current := issueNum
	maxDepth := 10 // Prevent infinite loops

	for depth := 0; depth < maxDepth; depth++ {
		if visited[current] {
			// Cycle detected, return current
			if verbose {
				fmt.Printf("  Warning: cycle detected in duplicate chain at #%d\n", current)
			}
			return current
		}
		visited[current] = true

		// Check if current issue is itself a duplicate
		if next, exists := duplicateMap[current]; exists && next != 0 {
			current = next
		} else {
			// Reached the root
			return current
		}
	}

	// Max depth reached
	if verbose {
		fmt.Printf("  Warning: max chain depth reached at #%d\n", current)
	}
	return current
}
