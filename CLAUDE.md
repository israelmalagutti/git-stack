# CLAUDE.md

## Project Overview

`gs` (git-stack) is a CLI tool for managing stacked branches in git, inspired by Graphite's `gt`.

## Key Documentation

- `docs/visualization-decisions.md` — Every major design decision and rationale behind `gs log` visualization. Read this before modifying the tree rendering code.
- `docs/mcp.md` — MCP server design: what MCP is, why gs uses it, tool reference, and architecture. Read this before modifying `cmd/mcp.go` or `internal/mcptools/`.
- `ai-context/mcp-server.md` — Full implementation plan with phases, data structures, and design decisions.
- `docs/branch-metadata-sync.md` — Ref-backed metadata sync design: why git refs, storage format, sync protocol, and team workflows. Read this before modifying `internal/config/ref_metadata.go` or `internal/git/refs.go`.

## Architecture

- `cmd/` — Cobra command definitions (one file per command)
- `cmd/mcp.go` — MCP server subcommand (`gs mcp`)
- `internal/mcptools/` — MCP tool handlers (read, write, navigation)
- `internal/stack/` — Stack tree model (`stack.go`) and visualization (`visualize.go`)
- `internal/colors/` — ANSI color system and terminal output helpers
- `internal/config/` — Config and metadata persistence
- `internal/config/ref_metadata.go` — Ref-backed metadata read/write (`refs/gs/meta/*`, `refs/gs/config`)
- `internal/git/` — Git repository operations wrapper
- `internal/git/refs.go` — Low-level git ref primitives (blob storage via `hash-object` / `update-ref`)
- `cmd/metadata_loader.go` — Metadata loading orchestration (ref-first with JSON fallback, auto-migration)

## Build & Test

- Language: Go
- Build: `go build -o gs .`
- Test: `go test ./...`
- Lint: handled by CI (golangci-lint)

## Stacking Mental Model (for MCP agents)

A "stack" is a tree of branches where each branch builds on its parent:

```
main (trunk)
├── feat/auth (depth 1, parent: main)
│   ├── feat/auth-tests (depth 2, parent: feat/auth)
│   └── feat/auth-docs (depth 2, parent: feat/auth)
└── feat/logging (depth 1, parent: main)
```

Key concepts:
- **Trunk**: The base branch (usually main/master). Cannot be deleted, renamed, or moved.
- **Parent**: The branch this branch was created on top of. Changes in the parent flow down during restack.
- **Depth**: Distance from trunk (trunk=0, its children=1, etc.).
- **Restack**: Rebasing a branch onto the current tip of its parent. Required after parent is modified.
- **Navigation**: "up" = toward leaves (children), "down" = toward trunk (parent).

The MCP server (`gs mcp`) exposes tools for programmatic stack management. Always call `gs_status` first to orient yourself. See `docs/mcp.md` for the full tool reference and workflow patterns.

## Visualization Invariants

- Every branch-out branch must be able to branch out on its own, in its own column. The column-based layout guarantees that any branch — regardless of depth — can have children that fork into new columns to its right. No branch is "locked" into a terminal position; the tree supports arbitrary nesting.
- Oldest child inherits parent's column; newer children get new columns to the right.
- Trunk is always column 0 and rendered separately at the bottom.
