# Contributing to Simili

First off, thank you for considering contributing to Simili! 🎉

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Developer Certificate of Origin (DCO)](#developer-certificate-of-origin-dco)
- [Getting Started](#getting-started)
- [How Can I Contribute?](#how-can-i-contribute)
- [Development Setup](#development-setup)
- [Pull Request Process](#pull-request-process)
- [Style Guidelines](#style-guidelines)

## Code of Conduct

This project adheres to a [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## Developer Certificate of Origin (DCO)

This project uses the [Developer Certificate of Origin (DCO)](https://developercertificate.org/) to ensure that all contributions are properly licensed. The DCO is enforced by the [DCO bot](https://github.com/apps/dco) on every pull request.

### Signing Off Your Commits

Every commit in your PR **must** include a `Signed-off-by` line. The easiest way to do this is with the `-s` flag:

```bash
git commit -s -m "feat: add awesome feature"
```

This appends a line like:

```
Signed-off-by: Your Name <your.email@example.com>
```

Make sure the name and email match your Git configuration:

```bash
git config user.name "Your Name"
git config user.email "your.email@example.com"
```

### Retroactive Sign-Off (Remediation)

If you submitted a PR **before** the DCO check was enabled, you do **not** need to rebase. You have two options:

#### Option 1: Reply to the bot's prompt

When the DCO check fails, a bot will comment on your PR asking if you'd like to sign.
Simply reply with the following exact phrase:

> I have read the DCO document and I hereby sign the DCO

The bot will then push a signed-off remediation commit to your branch automatically.

#### Option 2: Push a remediation commit manually

```bash
git commit --allow-empty -s -m "chore: retroactive DCO sign-off"
git push
```

A third-party (e.g. a maintainer) can also sign off on your behalf by pushing a remediation commit to your branch.

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/simili-bot.git
   cd simili-bot
   ```
3. **Add the upstream remote**:
   ```bash
   git remote add upstream https://github.com/similigh/simili-bot.git
   ```

## How Can I Contribute?

### Reporting Bugs

- Use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.yml)
- Check if the issue already exists
- Include as much detail as possible

### Suggesting Features

- Use the [feature request template](.github/ISSUE_TEMPLATE/feature_request.yml)
- Explain the problem you're trying to solve
- Describe your proposed solution

### Contributing Code

1. Look for issues labeled `good first issue` or `help wanted`
2. Comment on the issue to let others know you're working on it
3. Create a feature branch from `main`
4. Make your changes and commit with the `-s` flag (see [DCO](#developer-certificate-of-origin-dco))
5. Submit a pull request

## Development Setup

### Prerequisites

- Go 1.23+
- Git

### Building

```bash
# Build all packages
go build ./...

# Run tests
go test ./...

# Run linter
go vet ./...
```

### Project Structure

```
/cmd/simili         # CLI entry point
/internal/core      # Core packages (config, pipeline, state)
/internal/steps     # Pipeline steps (Lego blocks)
/internal/integrations  # External service clients
/DOCS               # Documentation
```

## Pull Request Process

1. **Create a feature branch**:
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make your changes** and commit with clear messages:
   ```bash
   git commit -m "feat: add awesome feature"
   ```
   
   We follow [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat:` - New feature
   - `fix:` - Bug fix
   - `docs:` - Documentation
   - `refactor:` - Refactoring
   - `test:` - Tests
   - `chore:` - Maintenance

3. **Keep your branch updated**:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

4. **Push and create a PR**:
   ```bash
   git push origin feature/my-feature
   ```

5. **Fill out the PR template** completely

6. **Address review feedback** promptly

## Style Guidelines

### Go Code

- Follow standard Go conventions
- Run `go fmt` before committing
- Run `go vet` to catch issues
- Keep functions focused and small
- Add comments for exported functions

### Commits

- Use [Conventional Commits](https://www.conventionalcommits.org/)
- Keep commits atomic and focused
- Write clear commit messages

### Documentation

- Update docs when changing behavior
- Use clear, concise language
- Include examples where helpful

---

Thank you for contributing! 🚀
