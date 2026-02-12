package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/google/go-github/v60/github"
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

type prDuplicateRunOptions struct {
	Token        string
	Org          string
	Repo         string
	Number       int
	TopK         int
	Threshold    float64
	PRCollection string
}

var prDuplicateCmd = &cobra.Command{
	Use:   "pr-duplicate",
	Short: "Check whether a PR is a duplicate of existing issues/PRs",
	Long: `Analyze a pull request for duplicate intent by searching both the issue
collection and PR collection, then using LLM analysis for a duplicate decision.`,
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

	cfg, err := loadPRDuplicateConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	opts, err := resolvePRDuplicateRunOptions(cfg)
	if err != nil {
		log.Fatalf("%v", err)
	}

	pr, prText, err := fetchPullRequestMetadataText(ctx, opts)
	if err != nil {
		log.Fatalf("Failed to fetch pull request metadata: %v", err)
	}

	embedding, err := generateEmbeddingForPRText(ctx, cfg, prText)
	if err != nil {
		log.Fatalf("Failed to embed pull request content: %v", err)
	}

	candidates, prCollectionMissing, err := findPRDuplicateCandidates(ctx, cfg, opts, embedding)
	if err != nil {
		log.Fatalf("Failed to search duplicate candidates: %v", err)
	}
	if prCollectionMissing && !prDuplicateJSON {
		fmt.Printf("Warning: PR collection '%s' does not exist; searching issues only.\n\n", opts.PRCollection)
	}

	duplicateResult, matched, err := detectPRDuplicate(ctx, cfg.Embedding.APIKey, pr, candidates)
	if err != nil {
		log.Fatalf("Failed to run duplicate analysis: %v", err)
	}

	out := buildPRDuplicateOutput(pr, opts, candidates, duplicateResult, matched)
	renderPRDuplicateOutput(out, cfg.Qdrant.Collection, opts.PRCollection, opts.Threshold)
}

func loadPRDuplicateConfig() (*config.Config, error) {
	cfgPath := config.FindConfigPath(cfgFile)
	if cfgPath == "" {
		return nil, fmt.Errorf("config file not found. Please verify your setup")
	}
	return config.Load(cfgPath)
}

func resolvePRDuplicateRunOptions(cfg *config.Config) (*prDuplicateRunOptions, error) {
	token := strings.TrimSpace(prDuplicateToken)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	if token == "" {
		return nil, fmt.Errorf("GitHub token is required (use --token or GITHUB_TOKEN env var)")
	}

	parts := strings.Split(prDuplicateRepo, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return nil, fmt.Errorf("invalid repo format: %s (expected owner/name)", prDuplicateRepo)
	}

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

	return &prDuplicateRunOptions{
		Token:        token,
		Org:          strings.TrimSpace(parts[0]),
		Repo:         strings.TrimSpace(parts[1]),
		Number:       prDuplicateNumber,
		TopK:         topK,
		Threshold:    threshold,
		PRCollection: resolvePRCollection(cfg, prDuplicatePRCollection),
	}, nil
}

func fetchPullRequestMetadataText(ctx context.Context, opts *prDuplicateRunOptions) (*github.PullRequest, string, error) {
	ghClient := similiGithub.NewClient(ctx, opts.Token)

	pr, err := ghClient.GetPullRequest(ctx, opts.Org, opts.Repo, opts.Number)
	if err != nil {
		return nil, "", fmt.Errorf("fetch pull request: %w", err)
	}

	filePaths, err := listAllPullRequestFilePaths(ctx, ghClient, opts.Org, opts.Repo, opts.Number)
	if err != nil {
		return nil, "", fmt.Errorf("fetch pull request files: %w", err)
	}

	return pr, buildPRMetadataText(pr, filePaths), nil
}

func generateEmbeddingForPRText(ctx context.Context, cfg *config.Config, prText string) ([]float32, error) {
	embedder, err := gemini.NewEmbedder(cfg.Embedding.APIKey, cfg.Embedding.Model)
	if err != nil {
		return nil, fmt.Errorf("initialize embedder: %w", err)
	}
	defer embedder.Close()

	embedding, err := embedder.Embed(ctx, prText)
	if err != nil {
		return nil, err
	}
	return embedding, nil
}

func findPRDuplicateCandidates(ctx context.Context, cfg *config.Config, opts *prDuplicateRunOptions, embedding []float32) ([]prDuplicateCandidate, bool, error) {
	qdrantClient, err := qdrant.NewClient(cfg.Qdrant.URL, cfg.Qdrant.APIKey)
	if err != nil {
		return nil, false, fmt.Errorf("initialize Qdrant client: %w", err)
	}
	defer qdrantClient.Close()

	searchLimit := opts.TopK * 3
	if searchLimit < opts.TopK {
		searchLimit = opts.TopK
	}

	issueCollectionExists, err := qdrantClient.CollectionExists(ctx, cfg.Qdrant.Collection)
	if err != nil {
		return nil, false, fmt.Errorf("verify issue collection '%s': %w", cfg.Qdrant.Collection, err)
	}
	if !issueCollectionExists {
		return nil, false, fmt.Errorf("issue collection '%s' does not exist", cfg.Qdrant.Collection)
	}

	issueResults, err := qdrantClient.Search(ctx, cfg.Qdrant.Collection, embedding, searchLimit, opts.Threshold)
	if err != nil {
		return nil, false, fmt.Errorf("search issue collection '%s': %w", cfg.Qdrant.Collection, err)
	}

	prResults := make([]*qdrant.SearchResult, 0)
	prCollectionMissing := false
	if opts.PRCollection != cfg.Qdrant.Collection {
		prCollectionExists, err := qdrantClient.CollectionExists(ctx, opts.PRCollection)
		if err != nil {
			return nil, false, fmt.Errorf("verify PR collection '%s': %w", opts.PRCollection, err)
		}
		if prCollectionExists {
			prResults, err = qdrantClient.Search(ctx, opts.PRCollection, embedding, searchLimit, opts.Threshold)
			if err != nil {
				return nil, false, fmt.Errorf("search PR collection '%s': %w", opts.PRCollection, err)
			}
		} else {
			prCollectionMissing = true
		}
	}

	candidates := mergeDuplicateCandidates(issueResults, prResults, opts.Org, opts.Repo, opts.Number)
	if len(candidates) > opts.TopK {
		candidates = candidates[:opts.TopK]
	}

	return candidates, prCollectionMissing, nil
}

func detectPRDuplicate(ctx context.Context, apiKey string, pr *github.PullRequest, candidates []prDuplicateCandidate) (*gemini.PRDuplicateResult, *prDuplicateCandidate, error) {
	if len(candidates) == 0 {
		return nil, nil, nil
	}

	llmClient, err := gemini.NewLLMClient(apiKey)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize LLM client: %w", err)
	}
	defer llmClient.Close()

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

	duplicateResult, err := llmClient.DetectPRDuplicate(ctx, &gemini.PRDuplicateCheckInput{
		PullRequest: &gemini.IssueInput{
			Title:  pr.GetTitle(),
			Body:   pr.GetBody(),
			Author: pr.GetUser().GetLogin(),
		},
		Candidates: llmCandidates,
	})
	if err != nil {
		return nil, nil, err
	}

	matched := findMatchedDuplicateCandidate(candidates, duplicateResult)
	return duplicateResult, matched, nil
}

func findMatchedDuplicateCandidate(candidates []prDuplicateCandidate, duplicateResult *gemini.PRDuplicateResult) *prDuplicateCandidate {
	if duplicateResult == nil || !duplicateResult.IsDuplicate || duplicateResult.DuplicateID == "" {
		return nil
	}

	for i := range candidates {
		if candidates[i].ID == duplicateResult.DuplicateID {
			c := candidates[i]
			return &c
		}
	}
	return nil
}

func buildPRDuplicateOutput(pr *github.PullRequest, opts *prDuplicateRunOptions, candidates []prDuplicateCandidate, duplicateResult *gemini.PRDuplicateResult, matched *prDuplicateCandidate) prDuplicateOutput {
	out := prDuplicateOutput{
		Candidates: candidates,
		Duplicate:  duplicateResult,
		Matched:    matched,
	}
	out.PullRequest.Org = opts.Org
	out.PullRequest.Repo = opts.Repo
	out.PullRequest.Number = opts.Number
	out.PullRequest.Title = pr.GetTitle()
	out.PullRequest.URL = pr.GetHTMLURL()
	return out
}

func renderPRDuplicateOutput(out prDuplicateOutput, issueCollection, prCollection string, threshold float64) {
	if prDuplicateJSON {
		printJSONOutput(out)
		return
	}

	fmt.Printf("PR: %s/%s#%d\n", out.PullRequest.Org, out.PullRequest.Repo, out.PullRequest.Number)
	fmt.Printf("Title: %s\n", out.PullRequest.Title)
	fmt.Printf("Issue Collection: %s\n", issueCollection)
	fmt.Printf("PR Collection: %s\n", prCollection)
	fmt.Printf("Threshold: %.2f\n\n", threshold)

	if len(out.Candidates) == 0 {
		fmt.Println("No similar issues or pull requests found.")
		return
	}

	fmt.Println("Top Candidates:")
	for i, c := range out.Candidates {
		label := "Issue"
		if c.EntityType == "pull_request" {
			label = "PR"
		}
		fmt.Printf("%d. [%s] %s/%s#%d (%.0f%%)\n", i+1, label, c.Org, c.Repo, c.Number, c.Similarity*100)
		fmt.Printf("   %s\n", c.Title)
		fmt.Printf("   %s\n", c.URL)
	}

	if out.Duplicate == nil {
		return
	}

	fmt.Println()
	if out.Duplicate.IsDuplicate {
		fmt.Printf("Duplicate: YES (confidence %.2f)\n", out.Duplicate.Confidence)
		if out.Matched != nil {
			label := "Issue"
			if out.Matched.EntityType == "pull_request" {
				label = "PR"
			}
			fmt.Printf("Matched: [%s] %s/%s#%d\n", label, out.Matched.Org, out.Matched.Repo, out.Matched.Number)
		} else if out.Duplicate.DuplicateID != "" {
			fmt.Printf("Matched ID: %s\n", out.Duplicate.DuplicateID)
		}
	} else {
		fmt.Printf("Duplicate: NO (confidence %.2f)\n", out.Duplicate.Confidence)
	}
	if out.Duplicate.Reasoning != "" {
		fmt.Printf("Reasoning: %s\n", out.Duplicate.Reasoning)
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
