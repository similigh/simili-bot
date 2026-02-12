# Setting Up Simili-Bot for Organizations

This guide describes how to configure Simili-Bot for an organization with multiple repositories. To minimize maintenance and ensure consistency, we recommend using GitHub Reusable Workflows.

## Overview

The recommended architecture consists of:
1.  **Central Configuration Repository**: A single repository (e.g., `shared-workflows` or `.github`) that hosts the shared workflow definition and configuration.
2.  **Caller Workflows**: Small workflow files in each repository that trigger the shared workflow.
3.  **Shared Vector Database**: A single Qdrant collection used by all repositories to enable cross-repository issue detection.

## Step 1: Create Central Resources

### 1. Central Repository
Create a public repository (or private, if using GitHub Enterprise/Organization Internal settings) to host your shared resources. For this guide, we will assume it is named `my-org/shared-config`.

### 2. Configuration File
Create a `.github/simili.yaml` file in your central repository. This file will define the organization-wide settings.

[View Example Configuration](./examples/multi-repo/simili.yaml)

### 3. Reusable Workflow
Create a `.github/workflows/simili.yml` file in your central repository. This is the logic that will be executed by all other repositories.

[View Example Shared Workflow](./examples/multi-repo/shared-workflow.yml)

## Step 2: Configure Secrets

Navigate to your Organization Settings > Secrets and variables > Actions. Add the following secrets at the organization level so they are accessible to all repositories:

- `GEMINI_API_KEY`: API key for Google Gemini.
- `QDRANT_URL`: URL of your Qdrant instance.
- `QDRANT_API_KEY`: API key for Qdrant authentication.

## Step 3: Configure Repositories

For each repository where you want to enable Simili-Bot, add a workflow file (e.g., `.github/workflows/issue-pr-triage.yml`) that calls the shared workflow.

[View Example Caller Workflow](./examples/multi-repo/caller-workflow.yml)

## Step 4: Configuration Inheritance (Optional)

You can allow individual repositories to override specific settings while inheriting the base configuration.

In a specific repository (e.g., `my-org/backend`), create a `.github/simili.yaml`:

```yaml
extends: "my-org/shared-config@main"

defaults:
  similarity_threshold: 0.85 # Stricter threshold for this repository
```

## Cross-Repository features

When configured correctly, Simili-Bot enables:
- **Duplicate Detection**: Identifying if a new issue/PR is a duplicate of an existing thread in *any* of the organization's repositories.
- **Unified Labeling**: Applying consistent labels across issues and pull requests.
