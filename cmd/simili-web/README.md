# Simili Web UI

A simple web interface for analyzing GitHub issues against the Simili Bot vector database.

## Overview

This web application provides a user-friendly interface to test issue analysis without making any GitHub writes. It processes issues through the same pipeline as the bot (similarity search, duplicate detection, quality assessment, and triage) but always runs in dry-run mode.

## Features

- **Issue Analysis**: Submit issue title and body to analyze against indexed issues
- **Similar Issues**: Find semantically similar issues with similarity scores
- **Duplicate Detection**: Check if the issue is a duplicate of existing issues
- **Quality Assessment**: Get quality scores and improvement suggestions
- **Label Suggestions**: Receive automated label recommendations
- **Transfer Recommendations**: See if the issue should be transferred to another repository
- **Minimal UI**: Clean, shadcn-inspired design with black/white theme and green/red/yellow accents

## Requirements

- **Go 1.21+**
- **Environment Variables**:
  - `GEMINI_API_KEY`: Optional AI key (takes precedence when both provider keys are set)
  - `OPENAI_API_KEY`: Optional AI key (used when Gemini key is not set)
  - `QDRANT_URL`: Vector database URL (e.g., `https://xxx.qdrant.io:6334`)
  - `QDRANT_API_KEY`: Qdrant authentication key
  - `QDRANT_COLLECTION`: Collection name (e.g., `ballerina-issues`)
  - `GITHUB_TOKEN`: Optional, for fetching additional issue data

## Quick Start

### 1. Build the application

```bash
cd cmd/simili-web
go build -o simili-web .
```

### 2. Set environment variables

```bash
export GEMINI_API_KEY="your-gemini-api-key"
# or export OPENAI_API_KEY="your-openai-api-key"
export QDRANT_URL="https://your-qdrant-instance.qdrant.io:6334"
export QDRANT_API_KEY="your-qdrant-api-key"
export QDRANT_COLLECTION="ballerina-issues"
```

### 3. Run the server

```bash
./simili-web
```

The server will start on `http://localhost:8080` by default.

### 4. Change port (optional)

```bash
PORT=3000 ./simili-web
```

## API Endpoints

### `GET /`
Serves the static web UI (HTML, CSS, JS)

### `GET /api/health`
Health check endpoint

**Response:**
```json
{
  "status": "ok"
}
```

### `POST /api/analyze`
Analyze an issue against the vector database

**Request Body:**
```json
{
  "title": "Bug: HTTP connector timeout issue",
  "body": "The HTTP connector times out after 30 seconds",
  "org": "ballerina-platform",
  "repo": "ballerina-library",
  "labels": ["bug", "module/http"]
}
```

**Response:**
```json
{
  "success": true,
  "similar_issues": [
    {
      "Number": 8634,
      "Title": "HTTP service hangs for request",
      "URL": "https://github.com/...",
      "Similarity": 0.85,
      "State": "open"
    }
  ],
  "is_duplicate": false,
  "duplicate_of": 0,
  "duplicate_reason": "Issues describe different problems",
  "quality_score": 0.85,
  "quality_issues": ["Missing error logs"],
  "suggested_labels": ["bug", "module/http", "Priority/High"],
  "transfer_target": "",
  "transfer_reason": ""
}
```

## Architecture

### Pipeline Steps

The web UI runs the following pipeline steps in order:

1. **Gatekeeper**: Validates the issue format and content
2. **Similarity Search**: Finds similar issues using vector embeddings
3. **Duplicate Detector**: Uses LLM to determine if issue is a duplicate
4. **Quality Checker**: Assesses issue quality and provides feedback
5. **Triage**: Suggests labels and transfer recommendations

**Note**: The `indexer` and `action_executor` steps are excluded to prevent any writes to the vector database or GitHub.

### Dry-Run Mode

All operations run with `DryRun=true`, ensuring:
- No comments posted to GitHub
- No labels applied
- No issue transfers
- No vector database writes

### Static Files

Static files (HTML, CSS, JS) are embedded into the binary using Go's `embed` package, making the application a single executable with no external dependencies.

## Configuration

The web UI respects the same `.simili.yaml` configuration file as the CLI. You can override settings via environment variables or by modifying the config file.

**Example `.simili.yaml`:**
```yaml
qdrant:
  url: https://your-instance.qdrant.io:6334
  api_key: ${QDRANT_API_KEY}
  collection: ballerina-issues

embedding:
  provider: gemini # or openai
  api_key: ${GEMINI_API_KEY} # or ${OPENAI_API_KEY}
  model: text-embedding-004 # or text-embedding-3-small
  dimensions: 768 # use 1536 for text-embedding-3-small

defaults:
  similarity_threshold: 0.75
  max_similar_to_show: 5
```

## Development

### Project Structure

```
cmd/simili-web/
├── main.go              # Server and API handlers
├── static/
│   ├── index.html       # Web UI structure
│   ├── styles.css       # Minimal shadcn-style CSS
│   └── app.js           # Frontend logic and API calls
└── README.md            # This file
```

### Adding New Features

1. **Backend**: Modify `main.go` to add new API endpoints or change pipeline steps
2. **Frontend**: Update `index.html`, `styles.css`, or `app.js` for UI changes
3. **Rebuild**: Run `go build` to embed updated static files into the binary

### Testing

Test the API directly with curl:

```bash
curl -X POST http://localhost:8080/api/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Bug: Test issue",
    "body": "This is a test",
    "org": "ballerina-platform",
    "repo": "ballerina-library"
  }'
```

## Troubleshooting

### "No AI API key found"
Set either `GEMINI_API_KEY` or `OPENAI_API_KEY` before running the server.

### "Failed to connect to Qdrant"
- Check `QDRANT_URL` format (include `https://` and port `:6334`)
- Verify `QDRANT_API_KEY` is correct
- Ensure Qdrant instance is accessible

### No similar issues found
- Verify the collection name matches your indexed data
- Check that issues have been indexed using `simili index`

### Port already in use
Change the port: `PORT=3001 ./simili-web`

## License

Part of the Simili Bot project.
