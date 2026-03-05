// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-03-05
// Last Modified: 2026-03-05

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	similiConfig "github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/integrations/ai"
	similiGithub "github.com/similigh/simili-bot/internal/integrations/github"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
)

var (
	prDupRepo      string
	prDupNumber    int
	prDupToken     string
	prDupDryRun    bool
	prDupTopK      int
	prDupThreshold float64
)

// PRDuplicateOutput is the JSON-serialisable result of the pr-duplicate command.
type PRDuplicateOutput struct {
	PR                *PRRef        `json:"pr"`
	Candidates        []PRCandidate `json:"candidates"`
	DuplicateDetected bool          `json:"duplicate_detected"`
	DuplicateOf       int           `json:"duplicate_of,omitempty"`
	Confidence        float64       `json:"confidence,omitempty"`
	Reasoning         string        `json:"reasoning,omitempty"`
}

// PRRef identifies the PR being checked.
type PRRef struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// PRCandidate represents a similar issue or PR found in the vector database.
type PRCandidate struct {
	Type   string  `json:"type"`
	Number int     `json:"number"`
	Title  string  `json:"title"`
	Score  float64 `json:"score"`
	URL    string  `json:"url"`
}

var prDuplicateCmd = &cobra.Command{
	Use:   "pr-duplicate",
	Short: "Detect duplicate PRs using semantic similarity",
	Long: `Fetch a pull request, embed its content, and search both the issues and PR
collections for semantically similar items. Outputs JSON candidates and
optionally runs an LLM-based duplicate verdict.`,
	Run: runPRDuplicate,
}

func init() {
	rootCmd.AddCommand(prDuplicateCmd)

	prDuplicateCmd.Flags().StringVar(&prDupRepo, "repo", "", "Target repository (owner/name)")
	prDuplicateCmd.Flags().IntVar(&prDupNumber, "number", 0, "PR number to check")
	prDuplicateCmd.Flags().StringVar(&prDupToken, "token", "", "GitHub token (defaults to GITHUB_TOKEN)")
	prDuplicateCmd.Flags().BoolVar(&prDupDryRun, "dry-run", false, "Simulate without querying Qdrant")
	prDuplicateCmd.Flags().IntVar(&prDupTopK, "top-k", 5, "Maximum candidates to return")
	prDuplicateCmd.Flags().Float64Var(&prDupThreshold, "threshold", 0.65, "Similarity score threshold")

	if err := prDuplicateCmd.MarkFlagRequired("repo"); err != nil {
		log.Fatalf("Failed to mark repo flag as required: %v", err)
	}
	if err := prDuplicateCmd.MarkFlagRequired("number"); err != nil {
		log.Fatalf("Failed to mark number flag as required: %v", err)
	}
}

func runPRDuplicate(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// 1. Parse repo.
	repoParts := strings.SplitN(prDupRepo, "/", 2)
	if len(repoParts) != 2 || repoParts[0] == "" || repoParts[1] == "" {
		log.Fatalf("Invalid repo format: %s (expected owner/name)", prDupRepo)
	}
	org, repoName := repoParts[0], repoParts[1]

	// 2. Load config.
	cfgPath := similiConfig.FindConfigPath(cfgFile)
	if cfgPath == "" {
		log.Fatalf("Config file not found. Please verify your setup.")
	}
	cfg, err := similiConfig.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 3. GitHub client.
	token := prDupToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		log.Fatal("GitHub token is required (use --token or GITHUB_TOKEN env var)")
	}
	gh := similiGithub.NewClient(ctx, token)

	// 4. Fetch PR details + changed files.
	pr, err := gh.GetPullRequest(ctx, org, repoName, prDupNumber)
	if err != nil {
		log.Fatalf("Failed to fetch PR #%d: %v", prDupNumber, err)
	}
	rawFiles, err := gh.ListPullRequestFiles(ctx, org, repoName, prDupNumber)
	if err != nil {
		log.Fatalf("Failed to fetch files for PR #%d: %v", prDupNumber, err)
	}
	filePaths := make([]string, 0, len(rawFiles))
	for _, f := range rawFiles {
		if p := f.GetFilename(); p != "" {
			filePaths = append(filePaths, p)
		}
	}

	out := &PRDuplicateOutput{
		PR: &PRRef{
			Repo:   prDupRepo,
			Number: prDupNumber,
			Title:  pr.GetTitle(),
		},
		Candidates: []PRCandidate{},
	}

	if prDupDryRun {
		printJSON(out)
		return
	}

	// 5. Embedder + embed PR content.
	embedder, err := ai.NewEmbedder(cfg.Embedding.APIKey, cfg.Embedding.Model)
	if err != nil {
		log.Fatalf("Failed to init embedder: %v", err)
	}
	defer embedder.Close()

	content := buildPREmbeddingContent(pr.GetTitle(), pr.GetBody(), filePaths)
	vec, err := embedder.Embed(ctx, content)
	if err != nil {
		log.Fatalf("Failed to embed PR content: %v", err)
	}

	// 6. Qdrant client.
	qdrantClient, err := qdrant.NewClient(cfg.Qdrant.URL, cfg.Qdrant.APIKey)
	if err != nil {
		log.Fatalf("Failed to init Qdrant: %v", err)
	}
	defer qdrantClient.Close()

	// 7. Search issues collection.
	issueHits, err := qdrantClient.Search(ctx, cfg.Qdrant.Collection, vec, prDupTopK, prDupThreshold)
	if err != nil {
		log.Printf("Warning: failed to search issues collection: %v", err)
		issueHits = nil
	}

	// 8. Search PR collection when configured.
	var prHits []*qdrant.SearchResult
	if cfg.Qdrant.PRCollection != "" {
		prHits, err = qdrantClient.Search(ctx, cfg.Qdrant.PRCollection, vec, prDupTopK, prDupThreshold)
		if err != nil {
			log.Printf("Warning: failed to search PR collection: %v", err)
		}
	}

	// 9. Merge, deduplicate, and sort candidates.
	out.Candidates = mergeSearchResults(issueHits, prHits, prDupNumber)

	// 10. Optional LLM duplicate verdict on top-3 candidates.
	llmKey := cfg.LLM.APIKey
	if llmKey == "" {
		llmKey = cfg.Embedding.APIKey
	}
	if llmKey != "" && len(out.Candidates) > 0 {
		llmClient, llmErr := ai.NewLLMClient(llmKey, cfg.LLM.Model)
		if llmErr == nil {
			defer llmClient.Close()

			top := out.Candidates
			if len(top) > 3 {
				top = top[:3]
			}
			similar := make([]ai.SimilarIssueInput, len(top))
			for i, c := range top {
				similar[i] = ai.SimilarIssueInput{
					Number:     c.Number,
					Title:      c.Title,
					URL:        c.URL,
					Similarity: c.Score,
				}
			}
			dupResult, dupErr := llmClient.DetectDuplicate(ctx, &ai.DuplicateCheckInput{
				CurrentIssue: &ai.IssueInput{
					Title: pr.GetTitle(),
					Body:  pr.GetBody(),
				},
				SimilarIssues: similar,
			})
			if dupErr == nil {
				out.DuplicateDetected = dupResult.IsDuplicate
				out.DuplicateOf = dupResult.DuplicateOf
				out.Confidence = dupResult.Confidence
				out.Reasoning = dupResult.Reasoning
			}
		}
	}

	printJSON(out)
}

// mergeSearchResults combines issue and PR search hits, deduplicates by (type, number),
// excludes the current PR itself, and sorts by score descending.
func mergeSearchResults(issueHits, prHits []*qdrant.SearchResult, currentPRNumber int) []PRCandidate {
	seen := make(map[string]struct{})
	var candidates []PRCandidate

	addHit := func(hit *qdrant.SearchResult) {
		itemType, _ := hit.Payload["type"].(string)
		if itemType == "" {
			itemType = "issue"
		}

		number := payloadInt(hit.Payload, "issue_number", "pr_number")
		title, _ := hit.Payload["title"].(string) //nolint:misspell
		url, _ := hit.Payload["url"].(string)

		// Exclude the PR being checked from its own results.
		if itemType == "pull_request" && number == currentPRNumber {
			return
		}

		key := fmt.Sprintf("%s:%d", itemType, number)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}

		candidates = append(candidates, PRCandidate{
			Type:   itemType,
			Number: number,
			Title:  title,
			Score:  float64(hit.Score),
			URL:    url,
		})
	}

	for _, hit := range issueHits {
		addHit(hit)
	}
	for _, hit := range prHits {
		addHit(hit)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	return candidates
}

// payloadInt extracts the first non-zero integer from the given payload keys.
func payloadInt(payload map[string]any, keys ...string) int {
	for _, k := range keys {
		switch v := payload[k].(type) {
		case int:
			if v != 0 {
				return v
			}
		case int64:
			if v != 0 {
				return int(v)
			}
		case float64:
			if v != 0 {
				return int(v)
			}
		}
	}
	return 0
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Fatalf("Failed to encode output: %v", err)
	}
}
