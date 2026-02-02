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
