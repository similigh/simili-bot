<p align="center">
  <img src="assets/logo.png" alt="Simili Logo" width="150">
</p>

# Simili Bot

<p align="center">
  <strong>AI-Powered GitHub Issue Intelligence</strong>
</p>

<p align="center">
  <a href="https://github.com/similigh/simili-bot/actions"><img src="https://img.shields.io/github/actions/workflow/status/similigh/simili-bot/ci.yml?branch=main&style=flat-square" alt="Build Status"></a>
  <a href="https://github.com/similigh/simili-bot/releases"><img src="https://img.shields.io/github/v/release/similigh/simili-bot?style=flat-square" alt="Release"></a>
  <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square" alt="License"></a>
  <a href="https://github.com/similigh/simili-bot"><img src="https://img.shields.io/github/stars/similigh/simili-bot?style=flat-square" alt="Stars"></a>
</p>

Automatically detect duplicate issues, find similar issues with semantic search, and intelligently route issues across repositories.

---

## Features

- **Semantic Duplicate Detection** — Find related issues using AI-powered embeddings, not just keyword matching.
- **Cross-Repository Search** — Search for similar issues across your organization.
- **Intelligent Routing** — Automatically transfer issues to the correct repository based on content.
- **Smart Triage** — AI-powered labeling and quality assessment.
- **Modular Pipeline** — Customize workflows with plug-and-play steps.
- **Multi-Repo Support** — Central configuration with per-repo overrides.

## Architecture

Simili uses a **"Lego with Blueprints"** architecture:

- **Lego Blocks**: Independent, reusable pipeline steps (Gatekeeper, Similarity, Triage, etc.).
- **Blueprints**: Pre-defined workflows for common use cases.
- **State Branch**: Git-based state management using an orphan branch (no comment scanning).

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Gatekeeper  │───▶│  Similarity │───▶│   Triage    │───▶│   Action    │
│   Check     │    │   Search    │    │  Analysis   │    │  Executor   │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

## Quick Start
Simili-Bot supports both **Single-Repository** and **Organization-wide** setups.

### Setup Guides

| Guide | Description |
|-------|-------------|
| [Single Repo Setup](DOCS/single-repo-setup.md) | Instructions for setting up Simili-Bot on a standalone repository. |
| [Organization Setup](DOCS/multi-repo-org-setup.md) | Best practices for deploying across an organization using Reusable Workflows. |

## Examples

We provide copy-pasteable examples to get you started quickly:

- **[Multi-Repo Examples](DOCS/examples/multi-repo)**: Includes shared workflow, caller workflow, and central config.
- **[Single-Repo Examples](DOCS/examples/single-repo)**: Standard workflow and configuration.

## Available Workflows

You can specify a `workflow` in your `simili.yaml` or define custom steps.

| Preset | Description |
|--------|-------------|
| `issue-triage` | Full pipeline: similarity search, duplicate check, triage analysis, and action execution. |
| `similarity-only` | Runs similarity search only. Useful for "Find Similar Issues" features without auto-triage. |
| `index-only` | Indexes issues to the vector database without providing feedback. |

## CLI Commands

Simili provides a powerful CLI for local development, testing, and batch operations.

### `simili index`

Bulk index issues from a GitHub repository into the vector database.

```bash
simili index --repo owner/repo --workers 5 --limit 100
```

Optionally index pull requests (metadata-only) into a separate PR collection.

```bash
simili index --repo owner/repo --workers 5 --include-prs
```

**Flags:**
- `--repo` (required): Target repository (owner/name)
- `--workers`: Number of concurrent workers (default: 5)
- `--since`: RFC3339 timestamp filter (uses GitHub `updated_at`)
- `--dry-run`: Simulate without writing to database
- `--include-prs`: Also index pull requests (metadata-only)
- `--pr-collection`: Override PR collection name (default: `qdrant.pr_collection` or `QDRANT_PR_COLLECTION`)

### `simili pr-duplicate`

Check whether a pull request appears to be a duplicate of existing issues or pull requests.
This command searches both the issue collection and PR collection, then runs an LLM duplicate decision.

```bash
simili pr-duplicate --repo owner/repo --number 123 --top-k 8
```

**Flags:**
- `--repo` (required): Target repository (owner/name)
- `--number` (required): Pull request number
- `--top-k`: Maximum combined candidates to evaluate (default: 8)
- `--threshold`: Similarity threshold override
- `--pr-collection`: Override PR collection name
- `--json`: Emit JSON output only

### `simili process`

Process a single issue through the pipeline.

```bash
simili process --issue issue.json --workflow issue-triage --dry-run
```

**Flags:**
- `--issue`: Path to issue JSON file
- `--workflow`: Workflow preset to run (default: "issue-triage")
- `--dry-run`: Run without side effects
- `--repo`, `--org`, `--number`: Override issue fields

### `simili batch`

Process multiple issues from a JSON file in batch mode. **All operations run in dry-run mode** to prevent GitHub writes.

```bash
simili batch --file issues.json --format csv --out-file results.csv --workers 5
```

**Use Cases:**
- Test bot logic on historical data without spamming repositories
- Generate reports showing similarity analysis and duplicate detection
- Analyze issues from repositories where you lack write access
- Bulk identify transfer recommendations and quality scores

**Flags:**
- `--file` (required): Path to JSON file with array of issues
- `--out-file`: Output file path (stdout if not specified)
- `--format`: Output format: `json` or `csv` (default: `json`)
- `--workers`: Number of concurrent workers (default: 1)
- `--workflow`: Workflow preset (default: "issue-triage")
- `--collection`: Override Qdrant collection name
- `--threshold`: Override similarity threshold
- `--duplicate-threshold`: Override duplicate confidence threshold
- `--top-k`: Override max similar issues to show

**Input Format:**

Create a JSON file with an array of issues:

```json
[
  {
    "org": "owner",
    "repo": "repo-name",
    "number": 123,
    "title": "Issue title",
    "body": "Issue description...",
    "state": "open",
    "labels": ["bug", "high-priority"],
    "author": "username",
    "created_at": "2026-02-10T10:00:00Z"
  }
]
```

**Output Formats:**

- **JSON**: Full pipeline results with detailed analysis
- **CSV**: Flattened summary for spreadsheet analysis

**Example Workflow:**

```bash
# 1. Index repository issues
simili index --repo ballerina-platform/ballerina-library --workers 10

# 2. Index PRs into separate collection
simili index --repo ballerina-platform/ballerina-library --workers 10 --include-prs

# 3. Check if a PR duplicates prior issues/PRs
simili pr-duplicate --repo ballerina-platform/ballerina-library --number 123 --top-k 10

# 4. Prepare test issues in batch.json
# 5. Run batch analysis
simili batch --file batch.json --format csv --out-file analysis.csv --workers 5

# 6. Review results
cat analysis.csv
```

## Development

```bash
# Clone the repository
git clone https://github.com/similigh/simili-bot.git
cd simili-bot

# Build
go build ./...

# Run tests
go test ./...

# Lint
go vet ./...
```

## License

This project is licensed under the Apache License 2.0 — see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  Made by the Simili Team
</p>
