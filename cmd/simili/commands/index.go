// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-13

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
	"github.com/similigh/simili-bot/internal/integrations/ai"
	similiGithub "github.com/similigh/simili-bot/internal/integrations/github"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
	"github.com/similigh/simili-bot/internal/utils/text"
	"github.com/spf13/cobra"
)

var (
	indexRepo       string
	indexSince      string // Can be a timestamp (ISO8601) or issue number (int)
	indexWorkers    int
	indexToken      string
	indexDryRun     bool
	indexIncludePRs bool
)

type Checkpoint struct {
	LastProcessedIssue int       `json:"last_processed_issue"`
	Timestamp          time.Time `json:"timestamp"`
}

// indexCmd represents the index command
var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Bulk index issues into the vector database",
	Long: `Index existing issues from a GitHub repository into the Qdrant vector database.
It fetches issues, comments, chunks the text, generates embeddings using the active AI provider,
and stores them for semantic search.

Supports resuming via a local checkpoint file or --since flag.`,
	Run: runIndex,
}

func init() {
	rootCmd.AddCommand(indexCmd)

	indexCmd.Flags().StringVar(&indexRepo, "repo", "", "Target repository (owner/name)")
	indexCmd.Flags().StringVar(&indexSince, "since", "", "Start indexing from this issue number or timestamp")
	indexCmd.Flags().IntVar(&indexWorkers, "workers", 5, "Number of concurrent workers")
	indexCmd.Flags().StringVar(&indexToken, "token", "", "GitHub token (optional, defaults to GITHUB_TOKEN env var)")
	indexCmd.Flags().BoolVar(&indexDryRun, "dry-run", false, "Simulate indexing without writing to DB")
	indexCmd.Flags().BoolVar(&indexIncludePRs, "include-prs", true, "Include pull requests in indexing")

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

	// 2. Auth & Clients
	token := indexToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		log.Fatal("GitHub token is required (use --token or GITHUB_TOKEN env var)")
	}

	ghClient := similiGithub.NewClient(ctx, token)

	embedder, err := ai.NewEmbedder(cfg.Embedding.APIKey, cfg.Embedding.Model)
	if err != nil {
		log.Fatalf("Failed to init embedder: %v", err)
	}
	defer embedder.Close()
	embeddingDimensions := cfg.Embedding.Dimensions
	if dim := embedder.Dimensions(); dim > 0 {
		embeddingDimensions = dim
	}

	var qdrantClient *qdrant.Client
	if !indexDryRun {
		qdrantClient, err = qdrant.NewClient(cfg.Qdrant.URL, cfg.Qdrant.APIKey)
		if err != nil {
			log.Fatalf("Failed to init Qdrant: %v", err)
		}
		defer qdrantClient.Close()

		// Ensure issues collection exists.
		if err = qdrantClient.CreateCollection(ctx, cfg.Qdrant.Collection, embeddingDimensions); err != nil {
			log.Fatalf("Failed to create/verify collection: %v", err)
		}
		// Ensure dedicated PR collection exists when configured.
		if cfg.Qdrant.PRCollection != "" {
			if err = qdrantClient.CreateCollection(ctx, cfg.Qdrant.PRCollection, embeddingDimensions); err != nil {
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

	log.Printf("Starting indexing for %s/%s with %d workers...", org, repoName, indexWorkers)

	splitter := text.NewRecursiveCharacterSplitter()

	type Job struct {
		Issue *github.Issue
	}

	jobs := make(chan Job, indexWorkers)
	var wg sync.WaitGroup

	// Issue workers.
	for i := 0; i < indexWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for job := range jobs {
				processIssue(ctx, id, job.Issue, ghClient, embedder, qdrantClient, splitter, cfg.Qdrant.Collection, org, repoName, indexDryRun)
			}
		}(i)
	}

	// PR workers — only when a dedicated PR collection is configured.
	var prJobs chan Job
	if indexIncludePRs && cfg.Qdrant.PRCollection != "" {
		prJobs = make(chan Job, indexWorkers)
		for i := 0; i < indexWorkers; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for job := range prJobs {
					processPullRequest(ctx, id, job.Issue, ghClient, embedder, qdrantClient, splitter, cfg.Qdrant.PRCollection, org, repoName, indexDryRun)
				}
			}(i)
		}
	}

	// Issue/PR producer.
	opts := &github.IssueListByRepoOptions{
		State:       "all",
		Sort:        "created",
		Direction:   "asc",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	if indexSince != "" {
		t, parseErr := time.Parse(time.RFC3339, indexSince)
		if parseErr == nil {
			opts.Since = t
		} else {
			log.Printf("Warning: Could not parse --since as ISO8601, ignoring (fetching all)")
		}
	}

	page := 1
	for {
		opts.Page = page
		issues, resp, err := ghClient.ListIssues(ctx, org, repoName, opts)
		if err != nil {
			log.Printf("Error listing issues page %d: %v", page, err)
			break
		}

		if len(issues) == 0 {
			break
		}

		log.Printf("Fetched page %d (%d issues)", page, len(issues))

		for _, issue := range issues {
			if !indexIncludePRs && issue.IsPullRequest() {
				continue
			}
			// Route PRs to the dedicated channel when available; otherwise fall
			// through to the issues collection (backward compatibility).
			if issue.IsPullRequest() && prJobs != nil {
				prJobs <- Job{Issue: issue}
			} else {
				jobs <- Job{Issue: issue}
			}
		}

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	close(jobs)
	if prJobs != nil {
		close(prJobs)
	}
	wg.Wait()
	log.Println("Indexing complete.")
}

// buildPREmbeddingContent builds the text that will be embedded for a pull request.
// Format: "Title: ...\n\nBody: ...\n\nChanged Files:\n- path/a\n- path/b"
func buildPREmbeddingContent(title, body string, files []string) string {
	var b strings.Builder
	b.WriteString("Title: ")
	b.WriteString(title)
	b.WriteString("\n\nBody: ")
	b.WriteString(body)
	if len(files) > 0 {
		b.WriteString("\n\nChanged Files:\n")
		for _, f := range files {
			b.WriteString("- ")
			b.WriteString(f)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// processPullRequest indexes a single pull request into the dedicated PR collection.
func processPullRequest(ctx context.Context, workerID int, issue *github.Issue, gh *similiGithub.Client, em *ai.Embedder, qd *qdrant.Client, splitter *text.RecursiveCharacterSplitter, prCollection, org, repo string, dryRun bool) {
	number := issue.GetNumber()

	// 1. Fetch full PR details.
	pr, err := gh.GetPullRequest(ctx, org, repo, number)
	if err != nil {
		log.Printf("[Worker %d] Error fetching PR #%d: %v", workerID, number, err)
		return
	}

	// 2. Fetch changed files.
	rawFiles, err := gh.ListPullRequestFiles(ctx, org, repo, number)
	if err != nil {
		log.Printf("[Worker %d] Error fetching files for PR #%d: %v", workerID, number, err)
		return
	}
	filePaths := make([]string, 0, len(rawFiles))
	for _, f := range rawFiles {
		if p := f.GetFilename(); p != "" {
			filePaths = append(filePaths, p)
		}
	}

	// 3. Build embedding content.
	content := buildPREmbeddingContent(pr.GetTitle(), pr.GetBody(), filePaths)

	// 4. Chunk.
	chunks := splitter.SplitText(content)

	// 5. Embed.
	embeddings, err := em.EmbedBatch(ctx, chunks)
	if err != nil {
		log.Printf("[Worker %d] Error embedding PR #%d: %v", workerID, number, err)
		return
	}

	// 6. Upsert.
	if dryRun {
		log.Printf("[DryRun] Would upsert PR #%d (%d chunks) to %s", number, len(chunks), prCollection)
		return
	}

	changedFiles := strings.Join(filePaths, ",")
	points := make([]*qdrant.Point, len(chunks))
	for i, chunk := range chunks {
		chunkID := uuid.NewMD5(uuid.NameSpaceURL, fmt.Appendf(nil, "%s/%s#PR%d-chunk-%d", org, repo, number, i)).String()
		points[i] = &qdrant.Point{
			ID:     chunkID,
			Vector: embeddings[i],
			Payload: map[string]any{
				"org":           org,
				"repo":          repo,
				"pr_number":     number,
				"text":          chunk,
				"url":           pr.GetHTMLURL(),
				"type":          "pull_request",
				"state":         pr.GetState(),
				"title":         pr.GetTitle(),
				"changed_files": changedFiles,
			},
		}
	}

	if err := qd.Upsert(ctx, prCollection, points); err != nil {
		log.Printf("[Worker %d] Error upserting PR #%d: %v", workerID, number, err)
	} else {
		log.Printf("[Worker %d] Indexed PR #%d", workerID, number)
	}
}

func processIssue(ctx context.Context, workerID int, issue *github.Issue, gh *similiGithub.Client, em *ai.Embedder, qd *qdrant.Client, splitter *text.RecursiveCharacterSplitter, collection, org, repo string, dryRun bool) {
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
	comments := make([]text.Comment, 0, len(allComments))
	for _, c := range allComments {
		body := strings.TrimSpace(c.GetBody())
		if body == "" {
			continue
		}
		author := "deleted-user"
		if c.User != nil {
			author = c.User.GetLogin()
		}
		comments = append(comments, text.Comment{Author: author, Body: body})
	}
	fullText := text.BuildEmbeddingContent(issue.GetTitle(), issue.GetBody(), comments)

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

	itemType := "issue"
	if issue.IsPullRequest() {
		itemType = "pull_request"
	}

	points := make([]*qdrant.Point, len(chunks))
	for i, chunk := range chunks {
		chunkID := uuid.NewMD5(uuid.NameSpaceURL, fmt.Appendf(nil, "%s/%s#%d-chunk-%d", org, repo, issue.GetNumber(), i)).String()
		points[i] = &qdrant.Point{
			ID:     chunkID,
			Vector: embeddings[i],
			Payload: map[string]any{
				"org":          org,
				"repo":         repo,
				"issue_number": issue.GetNumber(),
				"text":         chunk,
				"url":          issue.GetHTMLURL(),
				"type":         itemType,
				"state":        issue.GetState(),
				"title":        issue.GetTitle(),
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
