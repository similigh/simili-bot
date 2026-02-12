package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
	similiGithub "github.com/similigh/simili-bot/internal/integrations/github"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
	"github.com/spf13/cobra"
)

var (
	prDuplicateRepo         string
	prDuplicateNumber       int
	prDuplicateToken        string
	prDuplicatePRCollection string
	prDuplicateTopK         int
	prDuplicateThreshold    float64
	prDuplicateJSON         bool
)

type prDuplicateCandidate struct {
	ID         string  `json:"id"`
	EntityType string  `json:"entity_type"`
	Org        string  `json:"org"`
	Repo       string  `json:"repo"`
	Number     int     `json:"number"`
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	State      string  `json:"state"`
	Similarity float64 `json:"similarity"`
	Body       string  `json:"-"`
}

type prDuplicateOutput struct {
	PullRequest struct {
		Org    string `json:"org"`
		Repo   string `json:"repo"`
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
	} `json:"pull_request"`
	Candidates []prDuplicateCandidate    `json:"candidates"`
	Duplicate  *gemini.PRDuplicateResult `json:"duplicate,omitempty"`
	Matched    *prDuplicateCandidate     `json:"matched,omitempty"`
}

var prDuplicateCmd = &cobra.Command{
	Use:   "pr-duplicate",
	Short: "Check whether a PR is a duplicate of existing issues/PRs",
	Long: `Analyze a pull request for duplicate intent by searching both the issue
collection and PR collection, then using Gemini to make a duplicate decision.`,
	Run: runPRDuplicate,
}

func init() {
	rootCmd.AddCommand(prDuplicateCmd)

	prDuplicateCmd.Flags().StringVar(&prDuplicateRepo, "repo", "", "Target repository (owner/name)")
	prDuplicateCmd.Flags().IntVar(&prDuplicateNumber, "number", 0, "Pull request number")
	prDuplicateCmd.Flags().StringVar(&prDuplicateToken, "token", "", "GitHub token (optional, defaults to GITHUB_TOKEN env var)")
	prDuplicateCmd.Flags().StringVar(&prDuplicatePRCollection, "pr-collection", "", "Override PR collection name")
	prDuplicateCmd.Flags().IntVar(&prDuplicateTopK, "top-k", 8, "Maximum combined candidates to evaluate")
	prDuplicateCmd.Flags().Float64Var(&prDuplicateThreshold, "threshold", 0, "Similarity threshold override (default: config)")
	prDuplicateCmd.Flags().BoolVar(&prDuplicateJSON, "json", false, "Output JSON only")

	if err := prDuplicateCmd.MarkFlagRequired("repo"); err != nil {
		log.Fatalf("Failed to mark repo flag as required: %v", err)
	}
	if err := prDuplicateCmd.MarkFlagRequired("number"); err != nil {
		log.Fatalf("Failed to mark number flag as required: %v", err)
	}
}

func runPRDuplicate(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	cfgPath := config.FindConfigPath(cfgFile)
	if cfgPath == "" {
		log.Fatalf("Config file not found. Please verify your setup.")
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	token := prDuplicateToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		log.Fatal("GitHub token is required (use --token or GITHUB_TOKEN env var)")
	}

	parts := strings.Split(prDuplicateRepo, "/")
	if len(parts) != 2 {
		log.Fatalf("Invalid repo format: %s (expected owner/name)", prDuplicateRepo)
	}
	org, repo := parts[0], parts[1]

	threshold := prDuplicateThreshold
	if threshold <= 0 {
		threshold = cfg.Defaults.SimilarityThreshold
	}
	if threshold <= 0 {
		threshold = 0.65
	}

	topK := prDuplicateTopK
	if topK <= 0 {
		topK = cfg.Defaults.MaxSimilarToShow
	}
	if topK <= 0 {
		topK = 8
	}

	prCollection := resolvePRCollection(cfg, prDuplicatePRCollection)

	ghClient := similiGithub.NewClient(ctx, token)
	pr, err := ghClient.GetPullRequest(ctx, org, repo, prDuplicateNumber)
	if err != nil {
		log.Fatalf("Failed to fetch pull request: %v", err)
	}

	filePaths, err := listAllPullRequestFilePaths(ctx, ghClient, org, repo, prDuplicateNumber)
	if err != nil {
		log.Fatalf("Failed to fetch pull request files: %v", err)
	}

	prText := buildPRMetadataText(pr, filePaths)

	geminiKey := cfg.Embedding.APIKey
	if geminiKey == "" {
		geminiKey = os.Getenv("GEMINI_API_KEY")
	}
	if geminiKey == "" {
		log.Fatal("Gemini API key is required (set embedding.api_key or GEMINI_API_KEY)")
	}

	embedder, err := gemini.NewEmbedder(geminiKey, cfg.Embedding.Model)
	if err != nil {
		log.Fatalf("Failed to initialize Gemini embedder: %v", err)
	}
	defer embedder.Close()

	embedding, err := embedder.Embed(ctx, prText)
	if err != nil {
		log.Fatalf("Failed to embed pull request content: %v", err)
	}

	qdrantClient, err := qdrant.NewClient(cfg.Qdrant.URL, cfg.Qdrant.APIKey)
	if err != nil {
		log.Fatalf("Failed to initialize Qdrant client: %v", err)
	}
	defer qdrantClient.Close()

	searchLimit := topK * 3
	if searchLimit < topK {
		searchLimit = topK
	}

	issueCollectionExists, err := qdrantClient.CollectionExists(ctx, cfg.Qdrant.Collection)
	if err != nil {
		log.Fatalf("Failed to verify issue collection '%s': %v", cfg.Qdrant.Collection, err)
	}
	if !issueCollectionExists {
		log.Fatalf("Issue collection '%s' does not exist", cfg.Qdrant.Collection)
	}

	issueResults, err := qdrantClient.Search(ctx, cfg.Qdrant.Collection, embedding, searchLimit, threshold)
	if err != nil {
		log.Fatalf("Failed searching issue collection '%s': %v", cfg.Qdrant.Collection, err)
	}

	prResults := make([]*qdrant.SearchResult, 0)
	if prCollection != cfg.Qdrant.Collection {
		prCollectionExists, err := qdrantClient.CollectionExists(ctx, prCollection)
		if err != nil {
			log.Fatalf("Failed to verify PR collection '%s': %v", prCollection, err)
		}
		if prCollectionExists {
			prResults, err = qdrantClient.Search(ctx, prCollection, embedding, searchLimit, threshold)
			if err != nil {
				log.Fatalf("Failed searching PR collection '%s': %v", prCollection, err)
			}
		} else if !prDuplicateJSON {
			fmt.Printf("Warning: PR collection '%s' does not exist; searching issues only.\n\n", prCollection)
		}
	}

	candidates := mergeDuplicateCandidates(issueResults, prResults, org, repo, prDuplicateNumber)
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	llmClient, err := gemini.NewLLMClient(geminiKey)
	if err != nil {
		log.Fatalf("Failed to initialize Gemini LLM client: %v", err)
	}
	defer llmClient.Close()

	var duplicateResult *gemini.PRDuplicateResult
	var matched *prDuplicateCandidate
	if len(candidates) > 0 {
		llmCandidates := make([]gemini.PRDuplicateCandidateInput, len(candidates))
		for i, c := range candidates {
			llmCandidates[i] = gemini.PRDuplicateCandidateInput{
				ID:         c.ID,
				EntityType: c.EntityType,
				Org:        c.Org,
				Repo:       c.Repo,
				Number:     c.Number,
				Title:      c.Title,
				Body:       c.Body,
				URL:        c.URL,
				Similarity: c.Similarity,
				State:      c.State,
			}
		}

		duplicateResult, err = llmClient.DetectPRDuplicate(ctx, &gemini.PRDuplicateCheckInput{
			PullRequest: &gemini.IssueInput{
				Title:  pr.GetTitle(),
				Body:   pr.GetBody(),
				Author: pr.GetUser().GetLogin(),
			},
			Candidates: llmCandidates,
		})
		if err != nil {
			log.Fatalf("Failed to run duplicate analysis: %v", err)
		}

		if duplicateResult.IsDuplicate && duplicateResult.DuplicateID != "" {
			for i := range candidates {
				if candidates[i].ID == duplicateResult.DuplicateID {
					c := candidates[i]
					matched = &c
					break
				}
			}
		}
	}

	out := prDuplicateOutput{
		Candidates: candidates,
		Duplicate:  duplicateResult,
		Matched:    matched,
	}
	out.PullRequest.Org = org
	out.PullRequest.Repo = repo
	out.PullRequest.Number = prDuplicateNumber
	out.PullRequest.Title = pr.GetTitle()
	out.PullRequest.URL = pr.GetHTMLURL()

	if prDuplicateJSON {
		printJSONOutput(out)
		return
	}

	fmt.Printf("PR: %s/%s#%d\n", org, repo, prDuplicateNumber)
	fmt.Printf("Title: %s\n", pr.GetTitle())
	fmt.Printf("Issue Collection: %s\n", cfg.Qdrant.Collection)
	fmt.Printf("PR Collection: %s\n", prCollection)
	fmt.Printf("Threshold: %.2f\n\n", threshold)

	if len(candidates) == 0 {
		fmt.Println("No similar issues or pull requests found.")
		return
	}

	fmt.Println("Top Candidates:")
	for i, c := range candidates {
		label := "Issue"
		if c.EntityType == "pull_request" {
			label = "PR"
		}
		fmt.Printf("%d. [%s] %s/%s#%d (%.0f%%)\n", i+1, label, c.Org, c.Repo, c.Number, c.Similarity*100)
		fmt.Printf("   %s\n", c.Title)
		fmt.Printf("   %s\n", c.URL)
	}

	if duplicateResult == nil {
		return
	}

	fmt.Println()
	if duplicateResult.IsDuplicate {
		fmt.Printf("Duplicate: YES (confidence %.2f)\n", duplicateResult.Confidence)
		if matched != nil {
			label := "Issue"
			if matched.EntityType == "pull_request" {
				label = "PR"
			}
			fmt.Printf("Matched: [%s] %s/%s#%d\n", label, matched.Org, matched.Repo, matched.Number)
		} else if duplicateResult.DuplicateID != "" {
			fmt.Printf("Matched ID: %s\n", duplicateResult.DuplicateID)
		}
	} else {
		fmt.Printf("Duplicate: NO (confidence %.2f)\n", duplicateResult.Confidence)
	}
	if duplicateResult.Reasoning != "" {
		fmt.Printf("Reasoning: %s\n", duplicateResult.Reasoning)
	}
}

func printJSONOutput(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON output: %v", err)
	}
	fmt.Println(string(data))
}

func mergeDuplicateCandidates(issueResults, prResults []*qdrant.SearchResult, currentOrg, currentRepo string, currentPR int) []prDuplicateCandidate {
	byID := make(map[string]prDuplicateCandidate)

	add := func(res *qdrant.SearchResult) {
		candidate, ok := buildCandidateFromSearchResult(res)
		if !ok {
			return
		}

		if candidate.EntityType == "pull_request" &&
			candidate.Org == currentOrg &&
			candidate.Repo == currentRepo &&
			candidate.Number == currentPR {
			return
		}

		existing, found := byID[candidate.ID]
		if !found || candidate.Similarity > existing.Similarity {
			byID[candidate.ID] = candidate
		}
	}

	for _, res := range issueResults {
		add(res)
	}
	for _, res := range prResults {
		add(res)
	}

	merged := make([]prDuplicateCandidate, 0, len(byID))
	for _, candidate := range byID {
		merged = append(merged, candidate)
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Similarity > merged[j].Similarity
	})

	return merged
}

func buildCandidateFromSearchResult(res *qdrant.SearchResult) (prDuplicateCandidate, bool) {
	var candidate prDuplicateCandidate

	org, _ := res.Payload["org"].(string)
	repo, _ := res.Payload["repo"].(string)
	entityType, _ := res.Payload["type"].(string)

	if entityType == "" {
		if _, ok := res.Payload["pr_number"]; ok {
			entityType = "pull_request"
		} else {
			entityType = "issue"
		}
	}

	var number int
	var ok bool
	if entityType == "pull_request" {
		number, ok = toInt(res.Payload["pr_number"])
	} else {
		number, ok = toInt(res.Payload["issue_number"])
		if !ok {
			number, ok = toInt(res.Payload["number"])
		}
	}
	if !ok {
		return candidate, false
	}

	title, _ := res.Payload["title"].(string)
	body, _ := res.Payload["text"].(string)
	if body == "" {
		body, _ = res.Payload["description"].(string)
	}
	if title == "" {
		title = titleFromTextFallback(body)
	}
	if title == "" {
		title = "Untitled"
	}

	url, _ := res.Payload["url"].(string)
	state, _ := res.Payload["state"].(string)
	if state == "" {
		state = "open"
	}

	id := fmt.Sprintf("%s:%s/%s#%d", entityType, org, repo, number)
	candidate = prDuplicateCandidate{
		ID:         id,
		EntityType: entityType,
		Org:        org,
		Repo:       repo,
		Number:     number,
		Title:      title,
		URL:        url,
		State:      state,
		Similarity: float64(res.Score),
		Body:       body,
	}
	return candidate, true
}

func titleFromTextFallback(text string) string {
	if strings.HasPrefix(text, "Title: ") {
		lines := strings.SplitN(text, "\n", 2)
		if len(lines) > 0 {
			return strings.TrimSpace(strings.TrimPrefix(lines[0], "Title: "))
		}
	}
	return ""
}

func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	default:
		return 0, false
	}
}
