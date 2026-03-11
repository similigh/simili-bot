# Simili-Bot

Simili-Bot is a GitHub Action and CLI tool for AI-powered duplicate issue/PR detection
and cross-repo search. It is written in Go (1.23+).

## Architecture

- `cmd/` — CLI entry points (simili, simili-cli)
- `internal/` — Core logic (steps, config, embedding, LLM)
- `action.yml` — GitHub Action definition (Docker-based)
- `.github/simili.yaml` — Bot configuration

## Coding Standards

- Go 1.23+ with standard library conventions
- Run `go build ./...`, `go test ./...`, `go vet ./...` before committing
- Use golangci-lint for linting
- Follow existing patterns in `internal/` for new features
- Configuration uses YAML with environment variable interpolation (e.g., `${GEMINI_API_KEY}`)
