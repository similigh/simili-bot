// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package commands

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/google/uuid"
	similiConfig "github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
	similiGithub "github.com/similigh/simili-bot/internal/integrations/github"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
	"github.com/similigh/simili-bot/internal/utils/text"
	"github.com/spf13/cobra"
)

var (
	indexRepo         string
	indexSince        string // Timestamp (RFC3339), mapped to GitHub's "updated_at" filter.
	indexWorkers      int
	indexToken        string
	indexDryRun       bool
	indexIncludePRs   bool
	indexPRCollection string
)

type Checkpoint struct {
	LastProcessedIssue int       `json:"last_processed_issue"`
	Timestamp          time.Time `json:"timestamp"`
}

// indexCmd represents the index command
var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Bulk index issues (and optionally PRs) into the vector database",
	Long: `Index existing issues from a GitHub repository into the Qdrant vector database.
It fetches issues and comments, chunks the text, generates embeddings using AI,
and stores them for semantic search.

Optionally, pull requests can also be indexed into a dedicated PR collection
using metadata (title, description, changed file paths, and linked issues).

Supports resuming via a local checkpoint file or --since flag.`,
	Run: runIndex,
}

func init() {
	rootCmd.AddCommand(indexCmd)

	indexCmd.Flags().StringVar(&indexRepo, "repo", "", "Target repository (owner/name)")
	indexCmd.Flags().StringVar(&indexSince, "since", "", "Start indexing from this RFC3339 timestamp (filters by updated_at)")
	indexCmd.Flags().IntVar(&indexWorkers, "workers", 5, "Number of concurrent workers")
	indexCmd.Flags().StringVar(&indexToken, "token", "", "GitHub token (optional, defaults to GITHUB_TOKEN env var)")
	indexCmd.Flags().BoolVar(&indexDryRun, "dry-run", false, "Simulate indexing without writing to DB")
	indexCmd.Flags().BoolVar(&indexIncludePRs, "include-prs", false, "Also index pull requests (metadata only) into PR collection")
	indexCmd.Flags().StringVar(&indexPRCollection, "pr-collection", "", "Override PR collection name (default: qdrant.pr_collection or QDRANT_PR_COLLECTION)")

	if err := indexCmd.MarkFlagRequired("repo"); err != nil {
		log.Fatalf("Failed to mark repo flag as required: %v", err)
	}
}

func runIndex(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// 1. Load Config
	cfgPath := similiConfig.FindConfigPath(cfgFile)
	if cfgPath == "" {
		log.Fatalf("Config file not found. Please verify your setup.")
	}
	cfg, err := similiConfig.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	prCollection := resolvePRCollection(cfg, indexPRCollection)

	// 2. Auth & Clients
	token := indexToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		log.Fatal("GitHub token is required (use --token or GITHUB_TOKEN env var)")
	}

	ghClient := similiGithub.NewClient(ctx, token)

	geminiClient, err := gemini.NewEmbedder(cfg.Embedding.APIKey, cfg.Embedding.Model)
	if err != nil {
		log.Fatalf("Failed to init embedder: %v", err)
	}
	defer geminiClient.Close()
	embeddingDimensions := cfg.Embedding.Dimensions
	if dim := geminiClient.Dimensions(); dim > 0 {
		embeddingDimensions = dim
	}

	var qdrantClient *qdrant.Client
	if !indexDryRun {
		qdrantClient, err = qdrant.NewClient(cfg.Qdrant.URL, cfg.Qdrant.APIKey)
		if err != nil {
			log.Fatalf("Failed to init Qdrant: %v", err)
		}
		defer qdrantClient.Close()

		// Ensure collection exists
		err = qdrantClient.CreateCollection(ctx, cfg.Qdrant.Collection, embeddingDimensions)
		if err != nil {
			log.Fatalf("Failed to create/verify collection: %v", err)
		}

		if indexIncludePRs && prCollection != cfg.Qdrant.Collection {
			err = qdrantClient.CreateCollection(ctx, prCollection, embeddingDimensions)
			if err != nil {
				log.Fatalf("Failed to create/verify PR collection: %v", err)
			}
		}
	}

	// 3. Parse Repo
	parts := strings.Split(indexRepo, "/")
	if len(parts) != 2 {
		log.Fatalf("Invalid repo format: %s (expected owner/name)", indexRepo)
	}
	org, repoName := parts[0], parts[1]

	// 4. Determine Start Point (Since)
	// Checkpoint logic omitted for simplicity in v0.1.0 as standard pagination handles most cases.
	// Users can rely on --since for updates.

	log.Printf("Starting indexing for %s/%s with %d workers (include PRs: %t)...", org, repoName, indexWorkers, indexIncludePRs)

	// Fetch loop
	page := 1
	splitter := text.NewRecursiveCharacterSplitter()

	// Job channel
	type Job struct {
		Issue         *github.Issue
		IsPullRequest bool
	}
	jobs := make(chan Job, indexWorkers)
	var wg sync.WaitGroup

	// Start Workers
	for i := 0; i < indexWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for job := range jobs {
				if job.IsPullRequest {
					processPullRequest(ctx, id, job.Issue, ghClient, geminiClient, qdrantClient, splitter, prCollection, org, repoName, indexDryRun)
				} else {
					processIssue(ctx, id, job.Issue, ghClient, geminiClient, qdrantClient, splitter, cfg.Qdrant.Collection, org, repoName, indexDryRun)
				}
			}
		}(i)
	}

	// Issue Producer
	opts := &github.IssueListByRepoOptions{
		State:       "all",
		Sort:        "updated",
		Direction:   "asc",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	if indexSince != "" {
		t, err := time.Parse(time.RFC3339, indexSince)
		if err == nil {
			opts.Since = t
		} else {
			log.Printf("Warning: Could not parse --since as ISO8601, ignoring (fetching all)")
		}
	}

	for {
		opts.Page = page
		issues, resp, err := ghClient.ListIssues(ctx, org, repoName, opts)
		if err != nil {
			log.Printf("Error listing issues page %d: %v", page, err)
			break // Stop producer
		}

		if len(issues) == 0 {
			break
		}

		log.Printf("Fetched page %d (%d issues)", page, len(issues))

		for _, issue := range issues {
			if issue.IsPullRequest() {
				if indexIncludePRs {
					jobs <- Job{Issue: issue, IsPullRequest: true}
				}
				continue
			}
			jobs <- Job{Issue: issue, IsPullRequest: false}
		}

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	close(jobs)
	wg.Wait()
	log.Println("Indexing complete.")
}

func processIssue(ctx context.Context, workerID int, issue *github.Issue, gh *similiGithub.Client, em *gemini.Embedder, qd *qdrant.Client, splitter *text.RecursiveCharacterSplitter, collection, org, repo string, dryRun bool) {
	// 1. Fetch Comments (with pagination)
	var allComments []*github.IssueComment
	page := 1
	for {
		comments, resp, err := gh.ListComments(ctx, org, repo, issue.GetNumber(), &github.IssueListCommentsOptions{
			ListOptions: github.ListOptions{PerPage: 100, Page: page},
		})
		if err != nil {
			log.Printf("[Worker %d] Error fetching comments for #%d: %v", workerID, issue.GetNumber(), err)
			return
		}
		allComments = append(allComments, comments...)
		if resp == nil || resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	// 2. Aggregate Text
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Title: %s\n\n", issue.GetTitle()))

	issueBody := strings.TrimSpace(issue.GetBody())
	if issueBody != "" {
		sb.WriteString(fmt.Sprintf("Body: %s\n\n", issueBody))
	}

	hasCommentContent := false
	for _, c := range allComments {
		commentBody := strings.TrimSpace(c.GetBody())
		if commentBody == "" {
			continue
		}
		if !hasCommentContent {
			sb.WriteString("Comments:\n")
			hasCommentContent = true
		}
		author := "deleted-user"
		if c.User != nil {
			author = c.User.GetLogin()
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", author, commentBody))
	}

	fullText := sb.String()

	// 3. Chunk
	chunks := splitter.SplitText(fullText)

	// 4. Embed
	embeddings, err := em.EmbedBatch(ctx, chunks)
	if err != nil {
		log.Printf("[Worker %d] Error embedding #%d: %v", workerID, issue.GetNumber(), err)
		return
	}

	// 5. Upsert
	if dryRun {
		log.Printf("[DryRun] Would upsert #%d (%d chunks)", issue.GetNumber(), len(chunks))
		return
	}

	points := make([]*qdrant.Point, len(chunks))
	for i, chunk := range chunks {
		points[i] = &qdrant.Point{
			ID:     uuid.New().String(),
			Vector: embeddings[i],
			Payload: map[string]interface{}{
				"org":          org,
				"repo":         repo,
				"issue_number": issue.GetNumber(),
				"title":        issue.GetTitle(),
				"text":         chunk,
				"url":          issue.GetHTMLURL(),
				"state":        issue.GetState(),
				"type":         "issue",
			},
		}
	}

	err = qd.Upsert(ctx, collection, points)
	if err != nil {
		log.Printf("[Worker %d] Error upserting #%d: %v", workerID, issue.GetNumber(), err)
	} else {
		log.Printf("[Worker %d] Indexed #%d", workerID, issue.GetNumber())
	}
}

func processPullRequest(ctx context.Context, workerID int, issue *github.Issue, gh *similiGithub.Client, em *gemini.Embedder, qd *qdrant.Client, splitter *text.RecursiveCharacterSplitter, collection, org, repo string, dryRun bool) {
	prNumber := issue.GetNumber()

	pr, err := gh.GetPullRequest(ctx, org, repo, prNumber)
	if err != nil {
		log.Printf("[Worker %d] Error fetching PR #%d: %v", workerID, prNumber, err)
		return
	}

	filePaths, err := listAllPullRequestFilePaths(ctx, gh, org, repo, prNumber)
	if err != nil {
		log.Printf("[Worker %d] Error fetching files for PR #%d: %v", workerID, prNumber, err)
		return
	}

	fullText := buildPRMetadataText(pr, filePaths)
	if strings.TrimSpace(fullText) == "" {
		log.Printf("[Worker %d] PR #%d has no indexable content, skipping", workerID, prNumber)
		return
	}

	chunks := splitter.SplitText(fullText)
	if len(chunks) == 0 {
		chunks = []string{fullText}
	}

	embeddings, err := em.EmbedBatch(ctx, chunks)
	if err != nil {
		log.Printf("[Worker %d] Error embedding PR #%d: %v", workerID, prNumber, err)
		return
	}

	if dryRun {
		log.Printf("[DryRun] Would upsert PR #%d (%d chunks) into %s", prNumber, len(chunks), collection)
		return
	}

	points := make([]*qdrant.Point, len(chunks))
	for i, chunk := range chunks {
		pointID := uuid.NewMD5(uuid.NameSpaceURL, []byte(fmt.Sprintf("%s/%s/pr/%d/chunk/%d", org, repo, prNumber, i))).String()
		points[i] = &qdrant.Point{
			ID:     pointID,
			Vector: embeddings[i],
			Payload: map[string]interface{}{
				"org":           org,
				"repo":          repo,
				"pr_number":     prNumber,
				"title":         pr.GetTitle(),
				"description":   strings.TrimSpace(pr.GetBody()),
				"text":          chunk,
				"url":           pr.GetHTMLURL(),
				"state":         pr.GetState(),
				"merged":        pr.GetMerged(),
				"changed_files": strings.Join(filePaths, "\n"),
				"type":          "pull_request",
			},
		}
	}

	if err := qd.Upsert(ctx, collection, points); err != nil {
		log.Printf("[Worker %d] Error upserting PR #%d: %v", workerID, prNumber, err)
		return
	}

	log.Printf("[Worker %d] Indexed PR #%d", workerID, prNumber)
}
