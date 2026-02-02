# Simili-Bot v0.0.2v Implementation Plan: Foundation Integration

**Version:** 0.0.2v  
**Status:** 🚧 In Progress  
**Started:** 2026-02-02  
**Target Completion:** TBD

---

## Overview

This version transforms the scaffolded v0.0.1v architecture into a **functional bot** by integrating:

- **Gemini AI**: `gemini-embedding-001` for embeddings and `gemini-2.0-flash-lite` for LLM-based triage
- **Qdrant**: Vector database for semantic search
- **GitHub API**: Issue management and automation
- **CLI**: `simili process` command for local testing

**Architecture Philosophy:** Maintain the modular "Lego with Blueprints" design while wiring real integrations into the scaffolded steps.

---

## Goals

### Primary Goals
- ✅ Integrate Gemini embeddings (`gemini-embedding-001`, 768 dimensions)
- ✅ Integrate Gemini LLM (`gemini-2.0-flash-lite`) for triage analysis
- ✅ Integrate Qdrant vector store for semantic search
- ✅ Implement GitHub API client for issue operations
- ✅ Wire all integrations into existing pipeline steps
- ✅ Implement CLI `process` command for local testing
- ✅ Add file headers to all existing code

### Secondary Goals
- Add comprehensive unit tests for all integrations
- Create end-to-end integration tests
- Document testing procedures
- Ensure code follows Go best practices

---

## Implementation Phases

### Phase 1: Code Standards & File Headers ⏳
**Status:** Not Started  
**Estimated Effort:** 1 hour

- [ ] Add mandatory file headers to all existing `.go` files
- [ ] Ensure compliance with Go optimization guidelines
- [ ] Commit: `docs: add file headers to existing codebase`

**Files to Update:**
- All files in `cmd/simili/`
- All files in `internal/core/`
- All files in `internal/steps/`

---

### Phase 2: Gemini Integration ⏳
**Status:** Not Started  
**Estimated Effort:** 4-6 hours

#### Embedder Implementation
- [ ] Create `internal/integrations/gemini/` package
- [ ] Implement embedder using `gemini-embedding-001`
- [ ] Add unit tests for embedder
- [ ] Test with sample text (verify 768-dimensional output)

#### LLM Implementation
- [ ] Implement LLM client using `gemini-2.0-flash-lite`
- [ ] Create prompt templates for triage analysis
- [ ] Add unit tests for LLM client
- [ ] Test with sample issue data

**Dependencies:**
```go
github.com/google/generative-ai-go
```

**Commit:** `feat: implement Gemini embedder and LLM integration`

---

### Phase 3: Qdrant Vector Store Integration ⏳
**Status:** Not Started  
**Estimated Effort:** 3-4 hours

- [ ] Create `internal/integrations/qdrant/` package
- [ ] Implement VectorStore interface
- [ ] Add methods: CreateCollection, Upsert, Search, Delete
- [ ] Add unit tests with mocked Qdrant API
- [ ] Test connection to Qdrant instance

**Dependencies:**
```go
github.com/qdrant/go-client
```

**Commit:** `feat: implement Qdrant vector store integration`

---

### Phase 4: Wire Integrations to Pipeline Steps ⏳
**Status:** Not Started  
**Estimated Effort:** 3-4 hours

- [ ] Update `VectorDBPrep` step with real Qdrant client
- [ ] Update `SimilaritySearch` step with Gemini + Qdrant
- [ ] Update `Triage` step with Gemini LLM
- [ ] Update `Indexer` step with Gemini + Qdrant
- [ ] Add dependency injection to step constructors
- [ ] Test each step individually

**Commit:** `feat: wire Gemini and Qdrant into pipeline steps`

---

### Phase 5: GitHub API Client ⏳
**Status:** Not Started  
**Estimated Effort:** 3-4 hours

- [ ] Create `internal/integrations/github/` package
- [ ] Implement GitHub client for issue operations
- [ ] Add methods: GetIssue, CreateComment, AddLabels, TransferIssue
- [ ] Add authentication handling (token from env)
- [ ] Add unit tests with mocked GitHub API
- [ ] Test with real GitHub API (read-only operations)

**Dependencies:**
```go
github.com/google/go-github/v58
golang.org/x/oauth2
```

**Commit:** `feat: implement GitHub API client`

---

### Phase 6: Wire GitHub Client to Action Steps ⏳
**Status:** Not Started  
**Estimated Effort:** 2-3 hours

- [ ] Update `ResponseBuilder` step with GitHub client
- [ ] Update `ActionExecutor` step with GitHub client
- [ ] Add error handling for API rate limits
- [ ] Test comment posting (dry-run mode)
- [ ] Test label application (dry-run mode)

**Commit:** `feat: wire GitHub client to action steps`

---

### Phase 7: PendingActionScheduler Step ⏳
**Status:** Not Started  
**Estimated Effort:** 2-3 hours

- [ ] Implement `PendingActionScheduler` step
- [ ] Add logic to write pending actions to state branch
- [ ] Add expiry time calculation for pending transfers
- [ ] Add unit tests
- [ ] Test state branch writes

**Commit:** `feat: implement PendingActionScheduler step`

---

### Phase 8: CLI Process Command ⏳
**Status:** Not Started  
**Estimated Effort:** 3-4 hours

- [ ] Update `cmd/simili/main.go` with CLI framework
- [ ] Implement `process` command
- [ ] Add flags: --config, --dry-run, --issue-number, --org, --repo
- [ ] Wire up config loading and pipeline execution
- [ ] Add help text and usage examples
- [ ] Test CLI with sample issue JSON

**Commit:** `feat: implement CLI process command`

---

### Phase 9: Dependency Injection & Registry Updates ⏳
**Status:** Not Started  
**Estimated Effort:** 2 hours

- [ ] Update step registry to support dependency injection
- [ ] Inject clients into steps via constructors
- [ ] Ensure clean separation of concerns

**Commit:** `refactor: add dependency injection to step registry`

---

### Phase 10: End-to-End Integration Testing ⏳
**Status:** Not Started  
**Estimated Effort:** 4-5 hours

- [ ] Create test configuration file
- [ ] Set up test Qdrant collection
- [ ] Create test issue data
- [ ] Run full pipeline with CLI
- [ ] Verify embeddings are stored in Qdrant
- [ ] Verify similarity search returns results
- [ ] Verify LLM triage analysis works
- [ ] Document test results

**Commit:** `test: add end-to-end integration tests`

---

### Phase 11: Documentation & Release ⏳
**Status:** Not Started  
**Estimated Effort:** 2-3 hours

- [ ] Update README with v0.0.2v features
- [ ] Create CHANGELOG.md entry for v0.0.2v
- [ ] Update `.env.sample` if needed
- [ ] Create walkthrough document
- [ ] Tag release v0.0.2v

---

## Technical Decisions

### Gemini Models
- **Embeddings:** `gemini-embedding-001` (768 dimensions)
- **LLM:** `gemini-2.0-flash-lite` (fast, cost-effective)

**Rationale:** Free tier availability, good performance, official Google support.

### Qdrant Configuration
- **Collection:** Named per repository (e.g., `simili-bot-issues`)
- **Vector Dimension:** 768 (matches Gemini embeddings)
- **Distance Metric:** Cosine similarity

### GitHub API
- **Authentication:** Token from `GITHUB_TOKEN` environment variable
- **Rate Limiting:** Implement exponential backoff
- **Dry-Run Mode:** Log actions without executing for testing

---

## Testing Strategy

### Unit Tests
- Each integration package has comprehensive unit tests
- Mock external APIs for fast, reliable tests
- Target: >80% code coverage

### Integration Tests
- Test with real APIs (Gemini, Qdrant, GitHub)
- Use test collections/repositories
- Run in CI/CD pipeline

### Manual Testing
- CLI dry-run mode for safe testing
- Test with sample issues before production use

---

## Success Criteria

v0.0.2v is complete when:

- ✅ All existing files have proper headers
- ✅ Gemini embedder generates 768-dimensional vectors
- ✅ Gemini LLM performs triage analysis
- ✅ Qdrant stores and retrieves issue embeddings
- ✅ GitHub API client can fetch issues and post comments
- ✅ CLI `process` command executes full pipeline
- ✅ End-to-end test passes with real integrations
- ✅ All unit tests pass
- ✅ Code follows Go best practices

---

## Related Issues

- [#2: Implement Embedder Integration (Gemini/OpenAI)](https://github.com/similigh/simili-bot/issues/2)
- [#3: Implement VectorStore Integration (Qdrant)](https://github.com/similigh/simili-bot/issues/3)
- [#4: Implement GitHub API Client](https://github.com/similigh/simili-bot/issues/4)
- [#5: Implement PendingActionScheduler step](https://github.com/similigh/simili-bot/issues/5)
- [#6: CLI: Implement 'process' command](https://github.com/similigh/simili-bot/issues/6)

---

## Notes

- **Development Workflow:** Implement → Test → Commit (human-like development)
- **Code Standards:** Follow Go optimization pre-prompt guidelines
- **File Headers:** Mandatory for all `.go` files
- **Environment:** `.env` configured with necessary API keys

---

**Last Updated:** 2026-02-02
