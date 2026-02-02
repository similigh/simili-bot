# Multi-Repository Organization Setup

This guide explains how to set up Simili-Bot **across multiple repositories in an organization** with shared configuration and cross-repo issue intelligence.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Organization                            │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  my-org/simili-config (Central Config Repo)         │   │
│  │  └── .github/simili.yaml (Shared Config)            │   │
│  └─────────────────────────────────────────────────────┘   │
│            ▲              ▲              ▲                  │
│            │ extends      │ extends      │ extends          │
│  ┌─────────┴──┐   ┌──────┴──────┐   ┌───┴─────────┐        │
│  │  repo-a    │   │   repo-b    │   │   repo-c    │        │
│  │ (Backend)  │   │ (Frontend)  │   │   (Docs)    │        │
│  └────────────┘   └─────────────┘   └─────────────┘        │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Shared Qdrant Collection                │   │
│  │  (Cross-repo search: find duplicates org-wide)       │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## Step 1: Create Central Config Repository

Create a repository (e.g., `my-org/simili-config`) to hold the shared configuration.

### `.github/simili.yaml` (Central Config)

```yaml
# my-org/simili-config/.github/simili.yaml

qdrant:
  url: "${QDRANT_URL}"
  api_key: "${QDRANT_API_KEY}"
  collection: "my-org-issues"  # Shared collection for all repos

embedding:
  provider: "gemini"
  api_key: "${GEMINI_API_KEY}"

# Organization-wide workflow
workflow: "issue-triage"

defaults:
  similarity_threshold: 0.70
  max_similar_to_show: 5
  cross_repo_search: true  # Enable org-wide search

# Define all participating repositories
repositories:
  - org: "my-org"
    repo: "backend"
    enabled: true
    labels: ["backend", "api"]
  - org: "my-org"
    repo: "frontend"
    enabled: true
    labels: ["frontend", "ui"]
  - org: "my-org"
    repo: "docs"
    enabled: true
    labels: ["documentation"]
```

---

## Step 2: Add Org-Level Secrets

Go to **Organization Settings > Secrets and variables > Actions** and add:

| Secret Name      | Scope            |
| ---------------- | ---------------- |
| `GEMINI_API_KEY` | All repositories |
| `QDRANT_URL`     | All repositories |
| `QDRANT_API_KEY` | All repositories |

> **Note:** You can also use repository-specific secrets if needed.

---

## Step 3: Configure Each Repository

In each participating repository, create a minimal local config that **extends** the central one.

### `.github/simili.yaml` (Per-Repo Override)

```yaml
# my-org/backend/.github/simili.yaml

# Inherit from central config
extends: "my-org/simili-config@main"

# Optional: local overrides
defaults:
  similarity_threshold: 0.75  # Stricter for this repo
```

### `.github/workflows/simili.yml` (Same for all repos)

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

---

## Step 4: Enable Cross-Repo Transfers (Optional)

To allow issues to be automatically transferred between repositories:

1.  Create a **GitHub App** with `issues: write` on all target repos.
2.  Add the App ID and private key as secrets.
3.  Update the workflow:

```yaml
steps:
  - uses: actions/create-github-app-token@v1
    id: app-token
    with:
      app-id: ${{ vars.SIMILI_APP_ID }}
      private-key: ${{ secrets.SIMILI_PRIVATE_KEY }}

  - uses: similigh/simili-bot@v1
    with:
      config_path: .github/simili.yaml
      github_token: ${{ steps.app-token.outputs.token }}
    env:
      # ... other env vars
```

---

## Benefits of This Setup

| Feature              | Single Repo | Multi-Repo Org |
| -------------------- | ----------- | -------------- |
| Duplicate detection  | ✅ Same repo | ✅ Org-wide    |
| Shared config        | ❌          | ✅             |
| Cross-repo transfers | ❌          | ✅             |
| Central management   | ❌          | ✅             |

---

## Troubleshooting

### "Remote config not found"
Ensure the central config repo is accessible by the workflow. If private, the `GITHUB_TOKEN` must have access.

### "Cross-repo search not working"
Verify all repos use the **same Qdrant collection** name and that `cross_repo_search: true` is set.
