// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-05
// Last Modified: 2026-02-05

package commands

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	similiConfig "github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
	similiGithub "github.com/similigh/simili-bot/internal/integrations/github"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
	"github.com/spf13/cobra"
)

var (
	learnOrg    string
	learnRepo   string
	learnFile   string
	learnToken  string
	learnDryRun bool
)

// learnCmd represents the learn command
var learnCmd = &cobra.Command{
	Use:   "learn",
	Short: "Index repository documentation for hybrid routing",
	Long: `Learn repository documentation by fetching files from GitHub (README.md,
CONTRIBUTING.md, etc.) and indexing them into a separate Qdrant collection.

This enables the bot to understand what each repository is responsible for,
improving routing decisions even with zero historical issues (cold start).

Examples:
  simili learn --org my-org --repo backend --file README.md
  simili learn --org my-org --repo backend --file CONTRIBUTING.md
  simili learn --org my-org --repo backend --file docs/ARCHITECTURE.md --dry-run`,
	Run: runLearn,
}

func init() {
	rootCmd.AddCommand(learnCmd)

	learnCmd.Flags().StringVar(&learnOrg, "org", "", "Organization name (required)")
	learnCmd.Flags().StringVar(&learnRepo, "repo", "", "Repository name (required)")
	learnCmd.Flags().StringVar(&learnFile, "file", "README.md", "File path to learn (default: README.md)")
	learnCmd.Flags().StringVar(&learnToken, "token", "", "GitHub token (optional, defaults to GITHUB_TOKEN env var)")
	learnCmd.Flags().BoolVar(&learnDryRun, "dry-run", false, "Simulate without writing to database")

	if err := learnCmd.MarkFlagRequired("org"); err != nil {
		log.Fatalf("Failed to mark org flag as required: %v", err)
	}
	if err := learnCmd.MarkFlagRequired("repo"); err != nil {
		log.Fatalf("Failed to mark repo flag as required: %v", err)
	}
}

func runLearn(cmd *cobra.Command, args []string) {
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

	// 2. Initialize GitHub Client
	token := learnToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		log.Fatalf("GitHub token is required (use --token or GITHUB_TOKEN env var)")
	}

	ghClient := similiGithub.NewClient(ctx, token)

	// 3. Initialize Embedder
	embedder, err := gemini.NewEmbedder(cfg.Embedding.APIKey, cfg.Embedding.Model)
	if err != nil {
		log.Fatalf("Failed to initialize embedder: %v", err)
	}
	defer embedder.Close()

	// 4. Initialize Qdrant Client (unless dry-run)
	var qdrantClient *qdrant.Client
	if !learnDryRun {
		qdrantClient, err = qdrant.NewClient(cfg.Qdrant.URL, cfg.Qdrant.APIKey)
		if err != nil {
			log.Fatalf("Failed to initialize Qdrant client: %v", err)
		}
		defer qdrantClient.Close()
	}

	// 5. Validate and fetch file from GitHub
	// Clean path and validate (prevent path traversal)
	cleanPath := filepath.Clean(learnFile)
	if strings.Contains(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") {
		log.Fatalf("Invalid file path: %s (path traversal not allowed)", learnFile)
	}

	log.Printf("Fetching %s from %s/%s...", cleanPath, learnOrg, learnRepo)
	content, err := ghClient.GetFileContent(ctx, learnOrg, learnRepo, cleanPath, "")
	if err != nil {
		log.Fatalf("Failed to fetch %s from %s/%s: %v", cleanPath, learnOrg, learnRepo, err)
	}

	if len(content) == 0 {
		log.Printf("Warning: File %s is empty, skipping", learnFile)
		return
	}

	contentStr := string(content)
	log.Printf("Fetched %d characters from %s", len(contentStr), cleanPath)

	// 6. Generate Embedding
	log.Printf("Generating embedding...")
	embedding, err := embedder.Embed(ctx, contentStr)
	if err != nil {
		log.Fatalf("Failed to generate embedding: %v", err)
	}
	log.Printf("Generated embedding with %d dimensions", len(embedding))

	// 7. Create Point with Rich Payload
	// Use deterministic ID based on org/repo/file to enable idempotent updates
	idKey := fmt.Sprintf("%s/%s/%s", learnOrg, learnRepo, cleanPath)
	hash := sha256.Sum256([]byte(idKey))
	deterministicID := fmt.Sprintf("%x", hash[:16]) // Use first 16 bytes for UUID-like format

	point := &qdrant.Point{
		ID:     deterministicID,
		Vector: embedding,
		Payload: map[string]interface{}{
			"org":        learnOrg,
			"repo":       learnRepo,
			"file":       cleanPath,
			"text":       contentStr,
			"indexed_at": time.Now().Format(time.RFC3339),
			"type":       "repo_doc",
		},
	}

	// 8. Dry Run Check
	if learnDryRun {
		fmt.Printf("🔍 [DRY RUN] Would index %s/%s/%s\n", learnOrg, learnRepo, learnFile)
		fmt.Printf("📊 Collection: %s\n", cfg.Transfer.RepoCollection)
		fmt.Printf("📝 Content: %d characters\n", len(contentStr))
		fmt.Printf("🔢 Embedding: %d dimensions\n", len(embedding))
		return
	}

	// 9. Ensure Collection Exists
	repoCollection := cfg.Transfer.RepoCollection
	embeddingDimensions := cfg.Embedding.Dimensions
	if dim := embedder.Dimensions(); dim > 0 {
		embeddingDimensions = dim
	}
	log.Printf("Ensuring collection '%s' exists...", repoCollection)
	if err := qdrantClient.CreateCollection(ctx, repoCollection, embeddingDimensions); err != nil {
		log.Fatalf("Failed to create/verify collection: %v", err)
	}

	// 10. Upsert to Qdrant
	log.Printf("Indexing to collection '%s'...", repoCollection)
	if err := qdrantClient.Upsert(ctx, repoCollection, []*qdrant.Point{point}); err != nil {
		log.Fatalf("Failed to index document: %v", err)
	}

	// Success Output
	fmt.Printf("✅ Successfully indexed %s/%s/%s\n", learnOrg, learnRepo, learnFile)
	fmt.Printf("📊 Collection: %s\n", repoCollection)
	fmt.Printf("📝 Content: %d characters\n", len(contentStr))
	fmt.Printf("🔢 Embedding: %d dimensions\n", len(embedding))
}
