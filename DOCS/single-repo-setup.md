# Single Repository Setup

This guide explains how to set up Simili-Bot for a **single repository**.

## Prerequisites

1.  A GitHub repository
2.  A Qdrant Cloud account (free tier available: [cloud.qdrant.io](https://cloud.qdrant.io/))
3.  A Gemini API key (free tier available: [ai.google.dev](https://ai.google.dev/))

---

## Step 1: Add Secrets

Go to your repository's **Settings > Secrets and variables > Actions** and add:

| Secret Name      | Description                                |
| ---------------- | ------------------------------------------ |
| `GEMINI_API_KEY` | Your Gemini API key for embeddings         |
| `QDRANT_URL`     | Your Qdrant cluster URL (e.g., `https://...`) |
| `QDRANT_API_KEY` | Your Qdrant API key                        |

---

## Step 2: Create Configuration File

Create `.github/simili.yaml` in your repository:

```yaml
# .github/simili.yaml

qdrant:
  url: "${QDRANT_URL}"
  api_key: "${QDRANT_API_KEY}"
  collection: "my-repo-issues"  # Unique name for this repo

embedding:
  provider: "gemini"
  api_key: "${GEMINI_API_KEY}"

# Use a preset workflow for simplicity
workflow: "issue-triage"

defaults:
  similarity_threshold: 0.65
  max_similar_to_show: 5
```

---

## Step 3: Create Workflow File

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

---

## Step 4: Index Existing Issues (Optional)

If you have existing issues, you can index them using the CLI:

```bash
# Install the CLI
gh extension install similigh/simili-bot

# Index all open issues
gh simili index --repo owner/repo --config .github/simili.yaml
```

---

## Configuration Options

| Option                  | Description                          | Default |
| ----------------------- | ------------------------------------ | ------- |
| `similarity_threshold`  | Minimum similarity score (0-1)       | `0.65`  |
| `max_similar_to_show`   | Maximum similar issues to display    | `5`     |
| `workflow`              | Preset workflow name                 | `null`  |

See the [full configuration reference](./config-reference.md) for all options.
