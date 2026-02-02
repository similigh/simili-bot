# Simili-Bot v0.0.1 Implementation Plan

## 1. Architectural Strategy: "Lego with Blueprints"

To answer your question on modularity vs. ease of use, I propose a **hybrid approach**: The **"Lego with Blueprints"** architecture.

*   **The Lego Blocks (Core)**: The code will be built as small, independent, reusable **Steps** (e.g., `DuplicateChecker`, `Labeler`, `Responder`). Each step does one thing and does it well.
*   **The Blueprints (Config)**: We provide pre-defined "Workflows" (presets) for common use cases.
    *   **Beginners**: Use a `preset: "issue-triage"` in their YAML. They get a standard, best-practice flow.
    *   **Experts**: Can define `steps: [...]` list in YAML to mix and verify specific blocks, creating custom flows.

### Why this works for Multi-Repo/Org?
*   **Standardization**: An Org can define a "Corporate Standard" blueprint.
*   **Flexibility**: Specific teams can eject from the blueprint and customize their Lego blocks if needed.

---

## 2. Core Components

### 2.1 State Management: The "State Branch" (Fixing the Flaw)
Instead of scanning comments/labels, we will use a dedicated orphaned branch (e.g., `simili-state`) in the repository (or a central one).
*   **Storage Ref**: `refs/heads/simili-state`
*   **Structure**:
    ```text
    /pending
      /transfer
        /{issue_id}.json  <-- Contains target, expiry, metadata
      /close
        /{issue_id}.json
    ```
*   **Benefits**:
    *   **O(1) Lookup**: Checking if an issue has a pending action is just checking if a file exists.
    *   **Atomic**: Git operations are atomic.
    *   **Invisible**: Doesn't clutter the issue UI with comments.

### 2.2 The Pipeline Engine (The Lego Board)
A lightweight engine that takes a list of **Steps** and a **Context**.
*   **Interface**:
    ```go
    type Step interface {
        Name() string
        Run(ctx *Context) error
    }
    ```
*   **Context**: Holds the `Issue`, `Config`, and a `State` object (accumulated results from potential previous steps).

### 2.3 Configuration Scaling
To handle Org-wide rules:
*   **Remote Config**: The local `.simili.yaml` can point to a remote config: `extends: "my-org/simili-config@main"`.
*   **Merger**: The engine merges local overrides on top of the remote config.

---

## 3. Implementation Roadmap

### Phase 1: Foundation (The Bedrock) ✅
*   [x] **Config Engine**: Implement YAML loading with `extends` support for multi-repo inheritance.
*   [x] **Git State Manager**: Implement the driver to read/write JSON files to the `simili-state` branch without checking it out locally (using low-level git commands or go-git).

### Phase 2: The Core Pipeline (The Engine) ✅
*   [x] **Pipeline Runner**: The loop that executes steps.
*   [x] **Context Definition**: Efficient data structure to pass data between steps.
*   [x] **Step Registry**: Dynamic step factory registration with preset workflows.

### Phase 3: The Lego Blocks (The Features) ✅
*   [x] **Step Scaffolds**: Gatekeeper, VectorDBPrep, Similarity, TransferCheck, Triage, ResponseBuilder, ActionExecutor, Indexer.

---

## 4. Remaining Work (Future Phases)

> **Note**: These phases are planned for future implementation.

### Phase 4: Integrations
*   [ ] **Embedder Integration**: Wire up Gemini/OpenAI for embeddings.
*   [ ] **VectorStore Integration**: Connect to Qdrant.
*   [ ] **GitHub API Integration**: Comments, transfers, labels.

### Phase 5: State-Aware Blocks
*   [ ] **Step: `PendingActionScheduler`**: Writes to State Branch.
*   [ ] **Step: `PendingActionExecutor`**: Reads from State Branch and executes (Sync).

### Phase 6: Interface & Entry points
*   [ ] **CLI**: `simili process`, `simili sync`.
*   [ ] **GitHub Action**: Wrapper around the CLI.

---

## 5. Directory Structure
```text
/cmd
  /simili       # Main CLI entrypoint
/internal
  /core
    /pipeline   # The Engine
    /state      # Git State Manager
    /config     # Config Loader + Merger
  /steps        # The Lego Blocks (All logic goes here)
  /integrations # Clients (GitHub, Qdrant, Gemini)
```

