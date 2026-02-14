// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-13

// Package steps provides the indexer step for adding issues to the vector database.
package steps

import (
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v60/github"
	"github.com/google/uuid"
	"github.com/similigh/simili-bot/internal/core/pipeline"
	"github.com/similigh/simili-bot/internal/integrations/gemini"
	similiGithub "github.com/similigh/simili-bot/internal/integrations/github"
	"github.com/similigh/simili-bot/internal/integrations/qdrant"
	"github.com/similigh/simili-bot/internal/utils/text"
)

// Indexer adds/updates the issue in the vector database.
type Indexer struct {
	embedder *gemini.Embedder
	store    qdrant.VectorStore
	github   *similiGithub.Client
	dryRun   bool
}

// NewIndexer creates a new indexer step.
func NewIndexer(deps *pipeline.Dependencies) *Indexer {
	return &Indexer{
		embedder: deps.Embedder,
		store:    deps.VectorStore,
		github:   deps.GitHub,
		dryRun:   deps.DryRun,
	}
}

// Name returns the step name.
func (s *Indexer) Name() string {
	return "indexer"
}

// Run adds the issue to the vector database.
func (s *Indexer) Run(ctx *pipeline.Context) error {
	collectionName := ctx.Config.Qdrant.Collection

	// Skip indexing if the issue was transferred to another repository
	if ctx.Result.Transferred {
		log.Printf("[indexer] Issue transferred to %s, skipping indexing in source repo", ctx.Result.TransferTarget)
		return nil
	}

	if s.dryRun {
		log.Printf("[indexer] DRY RUN: Would index issue #%d into %s", ctx.Issue.Number, collectionName)
		return nil
	}

	if s.embedder == nil || s.store == nil {
		log.Printf("[indexer] WARNING: Missing dependencies, skipping indexing")
		return nil
	}

	// Fetch all comment pages for richer embedding content.
	var textComments []text.Comment
	if s.github != nil {
		page := 1
		for {
			ghComments, resp, err := s.github.ListComments(ctx.Ctx, ctx.Issue.Org, ctx.Issue.Repo, ctx.Issue.Number, &github.IssueListCommentsOptions{
				ListOptions: github.ListOptions{PerPage: 100, Page: page},
			})
			if err != nil {
				log.Printf("[indexer] WARNING: failed to fetch comments for #%d: %v", ctx.Issue.Number, err)
				break
			}
			for _, c := range ghComments {
				author := "deleted-user"
				if c.User != nil {
					author = c.User.GetLogin()
				}
				textComments = append(textComments, text.Comment{Author: author, Body: strings.TrimSpace(c.GetBody())})
			}
			if resp == nil || resp.NextPage == 0 {
				break
			}
			page = resp.NextPage
		}
	}

	// Create content for embedding
	content := text.BuildEmbeddingContent(ctx.Issue.Title, ctx.Issue.Body, textComments)

	// Generate embedding
	embedding, err := s.embedder.Embed(ctx.Ctx, content)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Resolve canonical item type for downstream consumers.
	itemType := "issue"
	if ctx.Issue.EventType == "pull_request" || ctx.Issue.EventType == "pr_comment" {
		itemType = "pull_request"
	}

	// Generate deterministic UUID
	uniqueID := fmt.Sprintf("%s-%s-%d", ctx.Issue.Org, ctx.Issue.Repo, ctx.Issue.Number)
	uuidID := uuid.NewMD5(uuid.NameSpaceURL, []byte(uniqueID)).String()

	// Prepare point for Qdrant
	point := &qdrant.Point{
		ID:     uuidID,
		Vector: embedding,
		Payload: map[string]any{
			"org":    ctx.Issue.Org,
			"repo":   ctx.Issue.Repo,
			"number": ctx.Issue.Number,
			"title":  ctx.Issue.Title,
			"url":    ctx.Issue.URL,
			"state":  ctx.Issue.State,
			"author": ctx.Issue.Author,
			"labels": ctx.Issue.Labels,
			"type":   itemType,
		},
	}

	// Upsert to Qdrant
	err = s.store.Upsert(ctx.Ctx, collectionName, []*qdrant.Point{point})
	if err != nil {
		return fmt.Errorf("failed to index issue: %w", err)
	}

	log.Printf("[indexer] Indexed issue #%d to %s", ctx.Issue.Number, collectionName)
	ctx.Result.Indexed = true

	return nil
}
