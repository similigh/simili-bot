# Setting Up Simili-Bot for a Single Repository

This guide details the steps to integrate Simili-Bot into a standalone repository.

## Prerequisites

- Access to the repository with permissions to manage workflows and secrets.
- At least one AI provider key:
  - **Google Gemini API Key** (`GEMINI_API_KEY`)
  - **OpenAI API Key** (`OPENAI_API_KEY`)
- A **Qdrant** instance (Cloud or self-hosted) for vector storage.

## Step 1: Configure Secrets

Navigate to **Settings > Secrets and variables > Actions** in your repository and add the following secrets:

- `GEMINI_API_KEY` (optional; takes precedence when both provider keys are set)
- `OPENAI_API_KEY` (optional; used when Gemini key is not set)
- `QDRANT_URL`
- `QDRANT_API_KEY`

You must set at least one of `GEMINI_API_KEY` or `OPENAI_API_KEY`.

## Step 2: Add Configuration

Create a file named `.github/simili.yaml` in your repository root.

The config should include both `embedding` and `llm` sections (LLM defaults to `gemini-2.0-flash-lite` if omitted).

[View Example Configuration](./examples/single-repo/simili.yaml)

## Step 3: Create Workflow

Create a GitHub Actions workflow file (e.g., `.github/workflows/simili.yml`) to trigger the bot on issue and pull request events.

[View Example Workflow](./examples/single-repo/workflow.yml)

## CLI For Backfilling

If you are adding Simili-Bot to a repository with existing issues, you can use the CLI to index them.

1.  **Install the Extension**:
    ```bash
    gh extension install similigh/simili-bot
    ```

2.  **Index Issues**:
    ```bash
    gh simili index --repo owner/repo --config .github/simili.yaml
    ```

## Step 4: Enable AI-Assisted Code Fixes (Optional)

Enable `@simili-bot` in PR comments for AI-powered code fixes using Claude Code.

### Prerequisites

- A Claude Pro or Max subscription
- The [Claude GitHub App](https://github.com/apps/claude) installed on your repo

### Setup

1. Generate your OAuth token locally:
   ```bash
   claude setup-token
   ```

2. Add `CLAUDE_CODE_OAUTH_TOKEN` as a repository secret in **Settings → Secrets → Actions**.

3. The example workflow already includes the Claude Code step. If you copied an older version, ensure your workflow has the conditional Claude Code step after `similigh/simili-bot@main`. See [the example workflow](./examples/single-repo/workflow.yml).

4. (Optional) Create a `CLAUDE.md` file in your repo root describing your project's architecture and coding standards. Claude Code reads this automatically for context.

### Usage

Comment on any PR (org members/collaborators only):

- `@simili-bot Fix the error handling issues` — uses latest Sonnet (default)
- `@simili-bot -opus Refactor this entire module` — uses Opus for complex tasks
