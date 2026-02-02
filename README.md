<p align="center">
  <img src="assets/logo.png" alt="Simili Logo" width="150">
</p>

<h1 align="center">Simili Bot</h1>

<p align="center">
  <strong>AI-Powered GitHub Issue Intelligence</strong>
</p>

<p align="center">
  <a href="https://github.com/similigh/simili-bot/actions"><img src="https://img.shields.io/github/actions/workflow/status/similigh/simili-bot/ci.yml?branch=main&style=flat-square" alt="Build Status"></a>
  <a href="https://github.com/similigh/simili-bot/releases"><img src="https://img.shields.io/github/v/release/similigh/simili-bot?style=flat-square" alt="Release"></a>
  <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square" alt="License"></a>
  <a href="https://github.com/similigh/simili-bot"><img src="https://img.shields.io/github/stars/similigh/simili-bot?style=flat-square" alt="Stars"></a>
</p>

<p align="center">
  Automatically detect duplicate issues, find similar issues with semantic search, and intelligently route issues across repositories.
</p>

---

## ✨ Features

- **🔍 Semantic Duplicate Detection** — Find related issues using AI-powered embeddings, not just keyword matching
- **🔄 Cross-Repository Search** — Search for similar issues across your organization
- **📦 Intelligent Routing** — Automatically transfer issues to the correct repository based on content
- **🏷️ Smart Triage** — AI-powered labeling and quality assessment
- **⚡ Modular Pipeline** — Customize workflows with plug-and-play steps
- **🌐 Multi-Repo Support** — Central configuration with per-repo overrides

## 🏗️ Architecture

Simili uses a **"Lego with Blueprints"** architecture:

- **Lego Blocks**: Independent, reusable pipeline steps (Gatekeeper, Similarity, Triage, etc.)
- **Blueprints**: Pre-defined workflows for common use cases
- **State Branch**: Git-based state management using an orphan branch (no comment scanning)

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Gatekeeper  │───▶│  Similarity │───▶│   Triage    │───▶│   Action    │
│   Check     │    │   Search    │    │  Analysis   │    │  Executor   │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

## 🚀 Quick Start

### Prerequisites

- [Qdrant](https://cloud.qdrant.io/) — Vector database (free tier available)
- [Gemini API Key](https://ai.google.dev/) — For embeddings (free tier available)

### 1. Add Secrets

Go to **Settings → Secrets and variables → Actions** and add:

| Secret | Description |
|--------|-------------|
| `GEMINI_API_KEY` | Gemini API key for embeddings |
| `QDRANT_URL` | Your Qdrant cluster URL |
| `QDRANT_API_KEY` | Qdrant API key |

### 2. Create Configuration

Create `.github/simili.yaml`:

```yaml
qdrant:
  url: "${QDRANT_URL}"
  api_key: "${QDRANT_API_KEY}"
  collection: "my-repo-issues"

embedding:
  provider: "gemini"
  api_key: "${GEMINI_API_KEY}"

workflow: "issue-triage"

defaults:
  similarity_threshold: 0.65
  max_similar_to_show: 5

# Optional: specify repositories (omit for single-repo mode)
repositories:
  - org: "your-org"
    repo: "your-repo"
    enabled: true
```

### 3. Add Workflow

Create `.github/workflows/simili.yml`:

```yaml
name: Simili Issue Intelligence

on:
  issues:
    types: [opened, edited, closed, reopened, deleted]

jobs:
  process:
    runs-on: ubuntu-latest
    permissions:
      issues: write
      contents: read

    steps:
      - uses: actions/checkout@v4

      - uses: similigh/simili-bot@v1
        with:
          config_path: .github/simili.yaml
        env:
          GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY }}
          QDRANT_URL: ${{ secrets.QDRANT_URL }}
          QDRANT_API_KEY: ${{ secrets.QDRANT_API_KEY }}
```

## 📚 Documentation

| Guide | Description |
|-------|-------------|
| [Single Repo Setup](DOCS/single-repo-setup.md) | Set up Simili for a single repository |
| [Multi-Repo Setup](DOCS/multi-repo-org-setup.md) | Organization-wide deployment with shared config |
| [Implementation Plan](DOCS/0.0.1v/plan.md) | Technical architecture and roadmap |

## 🔧 Available Workflows

| Preset | Description |
|--------|-------------|
| `issue-triage` | Full pipeline: similarity, transfer check, triage, actions |
| `similarity-only` | Find similar issues only, no triage |
| `index-only` | Just index issues to vector DB |

Or define custom steps:

```yaml
steps:
  - gatekeeper
  - similarity_search
  - response_builder
  - indexer
```

## 🛠️ Development

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

## 📄 License

This project is licensed under the Apache License 2.0 — see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  Made with ❤️ by the Simili Team
</p>
